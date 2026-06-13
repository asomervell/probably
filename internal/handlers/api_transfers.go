package handlers

import (
	"log/slog"
	"net/http"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// PendingMatch API response type
type PendingMatchResponse struct {
	ID              uuid.UUID           `json:"id"`
	TransactionID   uuid.UUID           `json:"transaction_id"`
	CandidateID     uuid.UUID           `json:"candidate_id"`
	ConfidenceScore float64             `json:"confidence_score"`
	Status          string              `json:"status"`
	MatchReasons    []string            `json:"match_reasons,omitempty"`
	Transaction     TransactionResponse `json:"transaction,omitempty"`
	Candidate       TransactionResponse `json:"candidate,omitempty"`
	CreatedAt       string              `json:"created_at"`
}

func pendingMatchToResponse(match *models.PendingTransferMatch) PendingMatchResponse {
	resp := PendingMatchResponse{
		ID:              match.ID,
		TransactionID:   match.TransactionID,
		CandidateID:     match.CandidateTransactionID,
		ConfidenceScore: match.ConfidenceScore,
		Status:          string(match.Status),
		MatchReasons:    match.MatchReasons,
		CreatedAt:       match.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if match.Transaction != nil {
		resp.Transaction = transactionToResponse(match.Transaction)
	}
	if match.CandidateTransaction != nil {
		resp.Candidate = transactionToResponse(match.CandidateTransaction)
	}

	return resp
}

// APITransfersPending returns all pending transfer matches for the current ledger
func (h *APIHandlers) APITransfersPending(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	matches, err := h.pendingMatches.GetPendingByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Load transaction details for each match (best-effort enrichment)
	for _, match := range matches {
		if err := h.pendingMatches.LoadTransactions(r.Context(), match, h.transactions); err != nil {
			slog.WarnContext(r.Context(), "failed to load pending match transactions", "match_id", match.ID, "err", err)
		}
		if match.Transaction != nil {
			if err := h.transactions.LoadEntries(r.Context(), match.Transaction); err != nil {
				slog.WarnContext(r.Context(), "failed to load entries", "txn_id", match.Transaction.ID, "err", err)
			}
		}
		if match.CandidateTransaction != nil {
			if err := h.transactions.LoadEntries(r.Context(), match.CandidateTransaction); err != nil {
				slog.WarnContext(r.Context(), "failed to load entries", "txn_id", match.CandidateTransaction.ID, "err", err)
			}
		}
	}

	// Convert to response format
	result := make([]PendingMatchResponse, len(matches))
	for i, match := range matches {
		result[i] = pendingMatchToResponse(match)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}

type manualMatchRequest struct {
	TransactionID1 uuid.UUID `json:"transaction_id_1"`
	TransactionID2 uuid.UUID `json:"transaction_id_2"`
}

// APITransfersManualMatch manually matches two transactions as a transfer
func (h *APIHandlers) APITransfersManualMatch(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	var req manualMatchRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.TransactionID1 == uuid.Nil || req.TransactionID2 == uuid.Nil {
		respondError(w, http.StatusBadRequest, "both transaction_id_1 and transaction_id_2 are required")
		return
	}

	if req.TransactionID1 == req.TransactionID2 {
		respondError(w, http.StatusBadRequest, "cannot match a transaction with itself")
		return
	}

	// Verify both transactions belong to this ledger
	txn1, err := h.transactions.GetByID(r.Context(), req.TransactionID1)
	if err != nil || txn1.LedgerID != ledger.ID {
		respondError(w, http.StatusBadRequest, "invalid transaction_id_1")
		return
	}

	txn2, err := h.transactions.GetByID(r.Context(), req.TransactionID2)
	if err != nil || txn2.LedgerID != ledger.ID {
		respondError(w, http.StatusBadRequest, "invalid transaction_id_2")
		return
	}

	// Check if either is already a transfer
	if txn1.IsTransfer || txn2.IsTransfer {
		respondError(w, http.StatusBadRequest, "one or both transactions are already marked as transfers")
		return
	}

	// Link them as a transfer pair
	if err := h.transactions.SetTransferPair(r.Context(), req.TransactionID1, req.TransactionID2); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	txn1, err = h.loadEnrichedTransaction(r.Context(), req.TransactionID1)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	txn2, err = h.loadEnrichedTransaction(r.Context(), req.TransactionID2)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"matched":      true,
		"transaction1": transactionToResponse(txn1),
		"transaction2": transactionToResponse(txn2),
	})
}

// APITransfersConfirm confirms a pending transfer match
func (h *APIHandlers) APITransfersConfirm(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	matchID, ok := mustAPIParamUUID(w, r, "id", "match ID")
	if !ok {
		return
	}

	match, err := h.pendingMatches.GetByID(r.Context(), matchID)
	if err != nil {
		respondError(w, http.StatusNotFound, "match not found")
		return
	}

	// Verify ownership by checking one of the transactions
	txn, err := h.transactions.GetByID(r.Context(), match.TransactionID)
	if err != nil || txn.LedgerID != ledger.ID {
		respondError(w, http.StatusNotFound, "match not found")
		return
	}

	if match.Status != models.MatchStatusPending {
		respondError(w, http.StatusBadRequest, "match is not pending")
		return
	}

	// Link the transactions as a transfer pair
	if err := h.transactions.SetTransferPair(r.Context(), match.TransactionID, match.CandidateTransactionID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update match status
	if err := h.pendingMatches.UpdateStatus(r.Context(), matchID, models.MatchStatusConfirmed); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	txn1, err := h.loadEnrichedTransaction(r.Context(), match.TransactionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	txn2, err := h.loadEnrichedTransaction(r.Context(), match.CandidateTransactionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"confirmed":    true,
		"transaction1": transactionToResponse(txn1),
		"transaction2": transactionToResponse(txn2),
	})
}

// APITransfersReject rejects a pending transfer match
func (h *APIHandlers) APITransfersReject(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	matchID, ok := mustAPIParamUUID(w, r, "id", "match ID")
	if !ok {
		return
	}

	match, err := h.pendingMatches.GetByID(r.Context(), matchID)
	if err != nil {
		respondError(w, http.StatusNotFound, "match not found")
		return
	}

	// Verify ownership by checking one of the transactions
	txn, err := h.transactions.GetByID(r.Context(), match.TransactionID)
	if err != nil || txn.LedgerID != ledger.ID {
		respondError(w, http.StatusNotFound, "match not found")
		return
	}

	if match.Status != models.MatchStatusPending {
		respondError(w, http.StatusBadRequest, "match is not pending")
		return
	}

	// Update match status
	if err := h.pendingMatches.UpdateStatus(r.Context(), matchID, models.MatchStatusRejected); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"rejected": true})
}

// APITransfersUnlink unlinks a confirmed transfer
func (h *APIHandlers) APITransfersUnlink(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	txnID, ok := mustAPIParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	txn, err := h.transactions.GetByID(r.Context(), txnID)
	if err != nil {
		respondError(w, http.StatusNotFound, "transaction not found")
		return
	}

	// Verify ownership
	if txn.LedgerID != ledger.ID {
		respondError(w, http.StatusNotFound, "transaction not found")
		return
	}

	if !txn.IsTransfer {
		respondError(w, http.StatusBadRequest, "transaction is not a transfer")
		return
	}

	// Unlink the transfer pair
	if err := h.transactions.UnlinkTransferPair(r.Context(), txnID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	txn, err = h.loadEnrichedTransaction(r.Context(), txnID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"unlinked":    true,
		"transaction": transactionToResponse(txn),
	})
}
