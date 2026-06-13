package sync

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/google/uuid"
)

// dollarsToCents converts a dollar amount (float64) to integer cents.
func dollarsToCents(dollars float64) int64 {
	return int64(math.Round(dollars * 100))
}

// logTransferMatchingError logs and captures a transfer-matching failure.
// Used by all three provider sync implementations which share identical error handling.
func logTransferMatchingError(ctx context.Context, component string, err error, txnID uuid.UUID) {
	slog.WarnContext(ctx, "transfer matching failed", "txn_id", txnID, "err", err)
	observability.CaptureFailure(ctx, err, observability.FailureOptions{
		Component: component,
		Operation: "transfer_matching",
		Tags:      map[string]string{"txn_id": txnID.String()},
	})
}

// updateLastSyncedAt stamps account.LastSyncedAt with the current time and persists it.
// Logs a warning on failure and a debug message on success.
func updateLastSyncedAt(ctx context.Context, account *models.Account, store *models.AccountStore) {
	now := time.Now()
	account.LastSyncedAt = &now
	if err := store.Update(ctx, account); err != nil {
		slog.WarnContext(ctx, "failed to update last synced timestamp", "account_id", account.ID, "err", err)
	} else {
		slog.DebugContext(ctx, "updated last_synced_at for account", "account", account.Name, "account_id", account.ID, "synced_at", now.Format(time.RFC3339))
	}
}
