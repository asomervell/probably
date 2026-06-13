package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

func (hdl *Handlers) AICategorizeBatch(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find all uncategorized transactions (those with no tags OR marked as done/failed with no tags)
	// This includes transactions that were processed but didn't get a tag
	filter := models.TransactionFilter{
		LedgerID: ledger.ID,
		Limit:    10000, // Get all uncategorized
	}
	isTransfer := false
	filter.IsTransfer = &isTransfer

	transactions, _, err := hdl.transactions.List(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find transactions without tags (regardless of categorization status)
	var uncategorizedIDs []uuid.UUID
	for _, txn := range transactions {
		if err := hdl.transactions.LoadTags(r.Context(), txn); err != nil {
			continue
		}
		if len(txn.Tags) == 0 {
			uncategorizedIDs = append(uncategorizedIDs, txn.ID)
		}
	}

	if len(uncategorizedIDs) == 0 {
		slog.InfoContext(r.Context(), "No uncategorized transactions found")
		// Redirect or HTMX response
		htmxRedirect(w, r, "/transactions")
		return
	}

	slog.InfoContext(r.Context(), "queueing uncategorized transactions for processing", "count", len(uncategorizedIDs))

	// Queue them for the unified processing worker
	if err := hdl.transactions.MarkForRecategorization(r.Context(), uncategorizedIDs); err != nil {
		slog.ErrorContext(r.Context(), "mark transactions for processing failed", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "queued transactions for processing", "count", len(uncategorizedIDs))

	// Redirect or HTMX response
	htmxRedirect(w, r, "/transactions")
}

// RecategorizeByTagName marks all transactions with a given tag name for recategorization
// POST /api/recategorize-by-tag with JSON body {"tag_name": "Other Income"}
func (hdl *Handlers) RecategorizeByTagName(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TagName == "" {
		http.Error(w, "tag_name is required", http.StatusBadRequest)
		return
	}

	// Mark all transactions with this tag for recategorization (across all ledgers)
	count, err := hdl.transactions.MarkAllForRecategorizationByTagName(r.Context(), req.TagName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Marked %d transactions for recategorization", count),
		"count":   count,
	})
}
