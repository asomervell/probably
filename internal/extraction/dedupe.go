package extraction

import (
	"context"
	"log/slog"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// CheckDuplicates filters out transactions that already exist in the database
// Returns only new transactions that don't already exist
func CheckDuplicates(ctx context.Context, transactionStore *models.TransactionStore, accountID uuid.UUID, transactions []ExtractedTransaction) ([]ExtractedTransaction, error) {
	var newTransactions []ExtractedTransaction
	skippedCount := 0

	for _, txn := range transactions {
		// Check if transaction already exists
		exists, err := transactionStore.ExistsByDateDescriptionAmount(ctx, accountID, txn.Date, txn.Description, txn.AmountCents)
		if err != nil {
			slog.WarnContext(ctx, "dedupe check error",
				"description", txn.Description, "date", txn.Date.Format("2006-01-02"), "error", err)
			// Continue processing other transactions even if one check fails
			continue
		}

		if exists {
			slog.DebugContext(ctx, "skipping duplicate transaction",
				"description", txn.Description, "date", txn.Date.Format("2006-01-02"), "amount_cents", txn.AmountCents)
			skippedCount++
			continue
		}

		newTransactions = append(newTransactions, txn)
	}

	if skippedCount > 0 {
		slog.InfoContext(ctx, "deduplication complete", "skipped", skippedCount, "new", len(newTransactions))
	}

	return newTransactions, nil
}

// ValidateTransactions validates and filters extracted transactions before processing.
// Skips those with empty descriptions, zero amounts, or zero dates.
func ValidateTransactions(ctx context.Context, transactions []ExtractedTransaction) []ExtractedTransaction {
	var validTransactions []ExtractedTransaction
	skippedCount := 0

	for i, txn := range transactions {
		skipReason := ""
		if txn.Description == "" {
			skipReason = "empty description"
		} else if txn.AmountCents == 0 {
			skipReason = "zero amount"
		} else if txn.Date.IsZero() {
			skipReason = "zero date"
		}

		if skipReason != "" {
			slog.DebugContext(ctx, "skipping invalid transaction",
				"index", i+1, "reason", skipReason,
				"date", txn.Date.Format("2006-01-02"), "description", txn.Description, "amount_cents", txn.AmountCents)
			skippedCount++
			continue
		}

		validTransactions = append(validTransactions, txn)
	}

	if skippedCount > 0 {
		slog.InfoContext(ctx, "transaction validation complete", "skipped", skippedCount, "valid", len(validTransactions))
	}

	return validTransactions
}
