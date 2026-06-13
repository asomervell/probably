package handlers

import (
	"net/http"
	"strconv"

	"github.com/asomervell/probably/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TransactionPattern represents a recurring pattern from transaction data
type TransactionPattern struct {
	EntityID        *uuid.UUID  `json:"entity_id,omitempty"`
	EntityName      string      `json:"entity_name,omitempty"`
	EntityLogo      string      `json:"entity_logo,omitempty"`
	PatternType     string      `json:"pattern_type"`
	PatternName     string      `json:"pattern_name,omitempty"`
	Frequency       string      `json:"frequency,omitempty"`
	AvgAmountCents  int64       `json:"avg_amount_cents"`
	Confidence      int         `json:"confidence"`
	IsSubscription  bool        `json:"is_subscription"`
	OccurrenceCount int         `json:"occurrence_count"`
	TransactionIDs  []uuid.UUID `json:"transaction_ids"`
}

// APIPatternsList returns all detected patterns for a ledger
// Patterns are aggregated from transactions with pattern_type = 'recurring_bill'
func (h *APIHandlers) APIPatternsList(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	// Query patterns aggregated by entity
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT 
			t.entity_id,
			COALESCE(e.name, t.description) as entity_name,
			COALESCE(e.logo_url, '') as entity_logo,
			t.pattern_type,
			COALESCE(t.pattern_metadata->>'pattern_name', '') as pattern_name,
			COALESCE(t.pattern_metadata->>'frequency', 'monthly') as frequency,
			AVG(ABS(en.amount_cents))::BIGINT as avg_amount_cents,
			AVG(COALESCE((t.pattern_metadata->>'confidence')::INT, 0))::INT as confidence,
			BOOL_OR(COALESCE((t.pattern_metadata->>'is_subscription')::BOOLEAN, false)) as is_subscription,
			COUNT(*) as occurrence_count,
			ARRAY_AGG(t.id ORDER BY t.date DESC) as transaction_ids
		FROM transactions t
		LEFT JOIN entities e ON t.entity_id = e.id
		LEFT JOIN entries en ON en.transaction_id = t.id AND en.amount_cents < 0
		WHERE t.ledger_id = $1
			AND t.pattern_type = 'recurring_bill'
			AND t.pattern_detection_status = 'done'
		GROUP BY t.entity_id, e.name, e.logo_url, t.pattern_type, 
			t.pattern_metadata->>'pattern_name', 
			t.pattern_metadata->>'frequency'
		ORDER BY avg_amount_cents DESC
	`, ledger.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var patterns []TransactionPattern
	for rows.Next() {
		var p TransactionPattern
		if err := rows.Scan(
			&p.EntityID, &p.EntityName, &p.EntityLogo, &p.PatternType,
			&p.PatternName, &p.Frequency, &p.AvgAmountCents, &p.Confidence,
			&p.IsSubscription, &p.OccurrenceCount, &p.TransactionIDs,
		); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		patterns = append(patterns, p)
	}
	if err := rows.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, patterns)
}

// APIPatternsGet returns pattern details for a specific transaction
func (h *APIHandlers) APIPatternsGet(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	txnID := chi.URLParam(r, "id")
	if txnID == "" {
		respondError(w, http.StatusBadRequest, "transaction ID required")
		return
	}

	txnUUID, err := uuid.Parse(txnID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid transaction ID")
		return
	}

	// Get transaction with pattern info
	txn, err := h.transactions.GetByID(r.Context(), txnUUID)
	if err != nil {
		respondError(w, http.StatusNotFound, "transaction not found")
		return
	}

	// Verify transaction belongs to this ledger
	if txn.LedgerID != ledger.ID {
		respondError(w, http.StatusForbidden, "transaction does not belong to this ledger")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":               txn.ID,
		"pattern_type":     txn.PatternType,
		"pattern_metadata": txn.PatternMetadata,
		"description":      txn.Description,
		"date":             txn.Date,
	})
}

// APIPatternsDetect manually triggers pattern detection for a ledger
func (h *APIHandlers) APIPatternsDetect(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	// Parse optional limit parameter
	limit := 100 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	transactions, err := h.transactions.GetQueuedForPatternDetection(r.Context(), ledger.ID, limit, 3)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(transactions) > 0 {
		ids := make([]uuid.UUID, len(transactions))
		for i, t := range transactions {
			ids[i] = t.ID
		}
		if err := h.transactions.QueueForPatternDetection(r.Context(), ids); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "pattern detection queued",
		"processed": len(transactions),
		"total":     len(transactions),
	})
}

// APIPatternsStats returns statistics about pattern detection for a ledger
func (h *APIHandlers) APIPatternsStats(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	var stats struct {
		Pending        int `json:"pending"`
		Queued         int `json:"queued"`
		Processing     int `json:"processing"`
		Done           int `json:"done"`
		Skipped        int `json:"skipped"`
		Failed         int `json:"failed"`
		RecurringBills int `json:"recurring_bills"`
		TotalPatterns  int `json:"total_patterns"`
	}

	// Get status counts
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT pattern_detection_status, COUNT(*) 
		FROM transactions 
		WHERE ledger_id = $1 
		GROUP BY pattern_detection_status
	`, ledger.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		switch status {
		case models.PatternDetectionStatusPending:
			stats.Pending = count
		case models.PatternDetectionStatusQueued:
			stats.Queued = count
		case models.PatternDetectionStatusProcessing:
			stats.Processing = count
		case models.PatternDetectionStatusDone:
			stats.Done = count
		case models.PatternDetectionStatusSkipped:
			stats.Skipped = count
		case models.PatternDetectionStatusFailed:
			stats.Failed = count
		}
	}
	if err := rows.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get pattern type counts
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT 
			COUNT(*) FILTER (WHERE pattern_type = 'recurring_bill') as recurring_bills,
			COUNT(*) FILTER (WHERE pattern_type IS NOT NULL AND pattern_type != 'none') as total_patterns
		FROM transactions 
		WHERE ledger_id = $1
	`, ledger.ID).Scan(&stats.RecurringBills, &stats.TotalPatterns)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, stats)
}
