package insights

import (
	"context"
	"fmt"
	"sync"
	"time"

	"log/slog"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Worker handles background insight generation
type Worker struct {
	cfg      *config.Config
	pool     *pgxpool.Pool
	reporter *Reporter
	analyzer *Analyzer
	ledgers  *models.LedgerStore
	insights *models.InsightStore

	// Worker control
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	running bool
}

// NewWorker creates a new insights worker
func NewWorker(cfg *config.Config, pool *pgxpool.Pool) *Worker {
	return &Worker{
		cfg:      cfg,
		pool:     pool,
		ledgers:  models.NewLedgerStore(pool),
		insights: models.NewInsightStore(pool),
		stopCh:   make(chan struct{}),
	}
}

// Start begins the background worker
func (w *Worker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	w.reporter = NewReporter(w.cfg, w.pool)
	w.analyzer = NewAnalyzer(w.cfg, w.pool)

	if !w.reporter.IsConfigured() && !w.analyzer.IsConfigured() {
		slog.Warn("no LLM providers configured for insights")
	}

	w.wg.Add(1)
	go w.run()

	slog.Info("insights worker started")
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopCh)
	w.wg.Wait()

	slog.Info("insights worker stopped")
}

// run is the main worker loop
func (w *Worker) run() {
	defer w.wg.Done()

	// Check for pending reports on startup
	w.checkPendingReports()

	// Schedule periodic checks
	reportTicker := time.NewTicker(1 * time.Hour)   // Check for pending reports hourly
	cleanupTicker := time.NewTicker(24 * time.Hour) // Cleanup old insights daily
	defer reportTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-reportTicker.C:
			w.checkPendingReports()
		case <-cleanupTicker.C:
			w.cleanupOldInsights()
		}
	}
}

// checkPendingReports checks all ledgers for missing periodic reports
func (w *Worker) checkPendingReports() {
	if !w.reporter.IsConfigured() {
		return
	}

	ctx := observability.WithDistinctID(context.Background(), "worker:probably-insights")

	// Get all ledgers
	ledgerIDs, err := w.getAllLedgerIDs(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get ledgers", "error", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "insights_worker",
			Operation: "get_ledgers",
		})
		return
	}

	var failedChecks int
	var firstFailure error

	for _, ledgerID := range ledgerIDs {
		// Check for pending reports
		pending, err := w.reporter.CheckPendingReports(ctx, ledgerID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to check pending reports", "ledger_id", ledgerID, "error", err)
			if firstFailure == nil {
				firstFailure = err
			}
			failedChecks++
			continue
		}

		// Generate missing reports
		for _, p := range pending {
			slog.DebugContext(ctx, "generating report", "type", p.ReportType, "ledger_id", p.LedgerID, "period", p.PeriodStart.Format("Jan 2006"))

			_, err := w.reporter.GeneratePendingReport(ctx, p)
			if err != nil {
				slog.ErrorContext(ctx, "failed to generate report", "error", err)
				if firstFailure == nil {
					firstFailure = err
				}
				failedChecks++
			} else {
				slog.DebugContext(ctx, "report generated")
			}

			// Rate limit between reports
			select {
			case <-w.stopCh:
				return
			case <-time.After(5 * time.Second):
			}
		}
	}

	if failedChecks > 0 {
		err := firstFailure
		if err == nil {
			err = fmt.Errorf("%d insight report operations failed", failedChecks)
		}
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "insights_worker",
			Operation: "check_pending_reports",
			Tags: map[string]string{
				"ledger_count":      fmt.Sprintf("%d", len(ledgerIDs)),
				"failed_operations": fmt.Sprintf("%d", failedChecks),
			},
		})
	}
}

// cleanupOldInsights removes old dismissed insights
func (w *Worker) cleanupOldInsights() {
	ctx := observability.WithDistinctID(context.Background(), "worker:probably-insights")

	// Delete dismissed insights older than 30 days
	cutoff := time.Now().AddDate(0, 0, -30)
	deleted, err := w.insights.CleanupOld(ctx, cutoff)
	if err != nil {
		slog.ErrorContext(ctx, "failed to cleanup old insights", "error", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "insights_worker",
			Operation: "cleanup_old_insights",
		})
		return
	}

	if deleted > 0 {
		slog.InfoContext(ctx, "cleaned up old dismissed insights", "count", deleted)
	}
}

// IsConfigured returns whether insight generation is available
func (w *Worker) IsConfigured() bool {
	if w.reporter == nil || w.analyzer == nil {
		return false
	}
	return w.reporter.IsConfigured() || w.analyzer.IsConfigured()
}

// getAllLedgerIDs retrieves all ledger IDs from the database
func (w *Worker) getAllLedgerIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := w.pool.Query(ctx, `SELECT id FROM ledgers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}
