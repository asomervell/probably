package sync

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/sync/providers"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncWorker periodically syncs transactions from all connected providers
type SyncWorker struct {
	syncService *SyncService
	accounts    *models.AccountStore

	// Worker control
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	running bool
}

// NewSyncWorker creates a new background sync worker for all providers
func NewSyncWorker(pool *pgxpool.Pool, cfg *config.Config) (*SyncWorker, error) {
	syncService := NewSyncService(cfg, pool)

	return &SyncWorker{
		syncService: syncService,
		accounts:    models.NewAccountStore(pool),
		stopCh:      make(chan struct{}),
	}, nil
}

// Start begins the background sync worker
func (w *SyncWorker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	w.wg.Add(1)
	go w.run()

	slog.Info("sync worker started", "schedule", "00,15,30,45")
}

// Stop gracefully stops the background sync worker
func (w *SyncWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopCh)
	w.wg.Wait()

	slog.Info("sync worker stopped")
}

func (w *SyncWorker) run() {
	defer w.wg.Done()

	// Sync at fixed times: 00, 15, 30, 45 minutes past each hour
	// This leaves API capacity for manual syncs
	syncMinutes := []int{0, 15, 30, 45}

	for {
		now := time.Now()
		nextSync := calculateNextSyncTime(now, syncMinutes)
		waitDuration := nextSync.Sub(now)

		slog.Debug("scheduled sync tick", "next_at", nextSync.Format("15:04"), "wait", waitDuration.Round(time.Second))

		timer := time.NewTimer(waitDuration)
		select {
		case <-w.stopCh:
			timer.Stop()
			return
		case <-timer.C:
			func() {
				defer observability.RecoverAndLog(context.Background(), "sync_worker_tick")
				w.syncAll()
			}()
		}
	}
}

// calculateNextSyncTime finds the next sync time based on allowed minutes
func calculateNextSyncTime(now time.Time, syncMinutes []int) time.Time {
	currentMinute := now.Minute()

	// Find the next sync minute in the current hour
	for _, m := range syncMinutes {
		if m > currentMinute {
			return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), m, 0, 0, now.Location())
		}
	}

	// No more sync times this hour, use first sync time of next hour
	nextHour := now.Add(time.Hour)
	return time.Date(nextHour.Year(), nextHour.Month(), nextHour.Day(), nextHour.Hour(), syncMinutes[0], 0, 0, now.Location())
}

// syncAll syncs all connected accounts from all providers
func (w *SyncWorker) syncAll() {
	ctx, cancel := context.WithTimeout(
		observability.WithDistinctID(context.Background(), "worker:probably-sync"),
		5*time.Minute,
	)
	defer cancel()

	startTime := time.Now()

	slog.InfoContext(ctx, "starting scheduled sync")

	accounts, err := w.accounts.GetAllWithProviderCredentials(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load connected accounts", "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "sync_worker",
			Operation: "load_connected_accounts",
		})
		return
	}

	if len(accounts) == 0 {
		slog.DebugContext(ctx, "no connected accounts, skipping sync")
		return
	}

	slog.DebugContext(ctx, "accounts to sync", "count", len(accounts))

	// Group accounts by provider and connection_id to optimize API calls
	byProviderAndConnection := make(map[string]map[string][]*models.Account)
	for _, acc := range accounts {
		provider := acc.Provider
		if provider == "" {
			provider = "teller"
		}
		connectionID := acc.ConnectionID
		if connectionID == "" {
			connectionID = acc.TellerEnrollmentID
		}

		if byProviderAndConnection[provider] == nil {
			byProviderAndConnection[provider] = make(map[string][]*models.Account)
		}
		byProviderAndConnection[provider][connectionID] = append(byProviderAndConnection[provider][connectionID], acc)
	}

	var totalSynced int
	var syncErrors int
	var transientErrors int
	var disconnected int
	var syncErrorDetails []string

	// Rate limit: wait between API calls (provider-specific delays can be added later)
	const apiDelay = 2 * time.Second

	for providerName, byConnection := range byProviderAndConnection {
		slog.DebugContext(ctx, "syncing provider", "provider", providerName)

		for connectionID, connectionAccounts := range byConnection {
			select {
			case <-w.stopCh:
				slog.InfoContext(ctx, "sync interrupted by shutdown")
				return
			default:
			}

			if len(connectionAccounts) == 0 {
				continue
			}

			slog.DebugContext(ctx, "syncing connection", "connection_id", connectionID, "accounts", len(connectionAccounts))

			connectionDisconnected := false
			for i, acc := range connectionAccounts {
				select {
				case <-w.stopCh:
					slog.InfoContext(ctx, "sync interrupted by shutdown")
					return
				default:
				}

				// Once one account in a connection is disconnected, skip remaining accounts —
				// they share the same token and will also fail, wasting API quota.
				if connectionDisconnected {
					disconnected += w.markAccountDisconnected(ctx, acc)
					continue
				}

				if i > 0 {
					time.Sleep(apiDelay)
				}

				synced, err := w.syncService.SyncTransactions(ctx, acc)
				if err != nil {
					if providers.IsConnectionDisconnectedError(err) {
						// Expected: token revoked or expired — not an application bug.
						slog.WarnContext(ctx, "connection disconnected, skipping remaining accounts", "connection_id", connectionID, "account", acc.Name, "account_id", acc.ID, "provider", providerName)
						disconnected += w.markAccountDisconnected(ctx, acc)
						connectionDisconnected = true
						continue
					}
					if IsProviderTransientError(err) {
						transientErrors++
						slog.WarnContext(ctx, "provider transient server error, will retry", "provider", providerName, "account", acc.Name, "account_id", acc.ID, "err", err)
						continue
					}
					syncErrors++
					syncErrorDetails = append(syncErrorDetails, fmt.Sprintf("%s/%s: %v", providerName, acc.Name, err))
					slog.ErrorContext(ctx, "account sync failed", "account", acc.Name, "account_id", acc.ID, "connection_id", connectionID, "provider", providerName, "err", err)
					observability.CaptureFailure(ctx, err, observability.FailureOptions{
						Component: "sync_worker",
						Operation: "sync_account",
						Tags: map[string]string{
							"account_id":    acc.ID.String(),
							"account_name":  acc.Name,
							"connection_id": connectionID,
							"provider":      providerName,
						},
					})
					continue
				}

				if synced > 0 {
					slog.DebugContext(ctx, "account synced", "account", acc.Name, "account_id", acc.ID, "transactions", synced)
					totalSynced += synced
				}
			}

			time.Sleep(apiDelay)
		}
	}

	totalConnections := countConnections(byProviderAndConnection)

	duration := time.Since(startTime)
	status := "success"
	if syncErrors > 0 {
		status = "partial_failure"
	} else if disconnected == totalConnections && totalConnections > 0 {
		status = "failure"
	}
	logFn := slog.InfoContext
	if syncErrors > 0 {
		logFn = slog.ErrorContext
	} else if status != "success" {
		logFn = slog.WarnContext
	}
	logFn(ctx, "provider sync complete", "trigger", "scheduled", "status", status, "connections", totalConnections, "transactions", totalSynced, "errors", syncErrors, "transient_errors", transientErrors, "disconnected", disconnected, "duration", duration.Round(time.Millisecond), "error_details", syncErrorDetails)
	captureProps := map[string]any{
		"trigger":          "scheduled",
		"sync_status":      status,
		"connections":      totalConnections,
		"transactions":     totalSynced,
		"errors":           syncErrors,
		"transient_errors": transientErrors,
		"disconnected":     disconnected,
		"duration_ms":      duration.Milliseconds(),
	}
	if len(syncErrorDetails) > 0 {
		captureProps["error_details"] = strings.Join(syncErrorDetails, "; ")
	}
	observability.CaptureEvent(ctx, "provider_sync_completed", captureProps)
}

// markAccountDisconnected sets the DB status and logs a warning. Returns 1 to increment a counter.
func (w *SyncWorker) markAccountDisconnected(ctx context.Context, acc *models.Account) int {
	if dbErr := w.accounts.SetConnectionStatus(ctx, acc.ID, "disconnected"); dbErr != nil {
		slog.WarnContext(ctx, "failed to mark account disconnected in db", "account_id", acc.ID, "err", dbErr)
		observability.CaptureFailure(ctx, dbErr, observability.FailureOptions{
			Component: "sync_worker",
			Operation: "mark_account_disconnected",
			Tags:      map[string]string{"account_id": acc.ID.String()},
		})
	}
	return 1
}

// countConnections returns the total number of unique connections across all providers.
func countConnections(byProviderAndConnection map[string]map[string][]*models.Account) int {
	total := 0
	for _, byConnection := range byProviderAndConnection {
		total += len(byConnection)
	}
	return total
}

