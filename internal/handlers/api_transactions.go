package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// Transaction API response types
type EntryResponse struct {
	ID          uuid.UUID          `json:"id"`
	AccountID   uuid.UUID          `json:"account_id"`
	AccountName string             `json:"account_name,omitempty"`
	AccountType models.AccountType `json:"account_type,omitempty"`
	AmountCents int64              `json:"amount_cents"`
	Currency    string             `json:"currency"`
}

type TagResponse struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	Color    string     `json:"color"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

type TransactionResponse struct {
	ID               uuid.UUID       `json:"id"`
	LedgerID         uuid.UUID       `json:"ledger_id"`
	Date             string          `json:"date"`
	Description      string          `json:"description"`
	Notes            string          `json:"notes,omitempty"`
	IsTransfer       bool            `json:"is_transfer"`
	TransferPairID   *uuid.UUID      `json:"transfer_pair_id,omitempty"`
	TellerCategory   string          `json:"teller_category,omitempty"`
	CounterpartyName string          `json:"counterparty_name,omitempty"`
	Entries          []EntryResponse `json:"entries,omitempty"`
	Tags             []TagResponse   `json:"tags,omitempty"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
}

func transactionToResponse(txn *models.Transaction) TransactionResponse {
	resp := TransactionResponse{
		ID:               txn.ID,
		LedgerID:         txn.LedgerID,
		Date:             txn.Date.Format("2006-01-02"),
		Description:      txn.Description,
		Notes:            txn.Notes,
		IsTransfer:       txn.IsTransfer,
		TransferPairID:   txn.TransferPairID,
		TellerCategory:   txn.TellerCategory,
		CounterpartyName: txn.CounterpartyName,
		CreatedAt:        txn.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:        txn.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if txn.Entries != nil {
		resp.Entries = make([]EntryResponse, len(txn.Entries))
		for i, e := range txn.Entries {
			resp.Entries[i] = EntryResponse{
				ID:          e.ID,
				AccountID:   e.AccountID,
				AccountName: e.AccountName,
				AccountType: e.AccountType,
				AmountCents: e.AmountCents,
				Currency:    e.Currency,
			}
		}
	}

	if txn.Tags != nil {
		resp.Tags = make([]TagResponse, len(txn.Tags))
		for i, t := range txn.Tags {
			resp.Tags[i] = TagResponse{
				ID:       t.ID,
				Name:     t.Name,
				Color:    t.Color,
				ParentID: t.ParentID,
			}
		}
	}

	return resp
}

func (h *APIHandlers) enrichTransaction(ctx context.Context, txn *models.Transaction) {
	if err := h.transactions.LoadEntries(ctx, txn); err != nil {
		slog.WarnContext(ctx, "LoadEntries failed", "transaction_id", txn.ID, "err", err)
	}
	if err := h.transactions.LoadTags(ctx, txn); err != nil {
		slog.WarnContext(ctx, "LoadTags failed", "transaction_id", txn.ID, "err", err)
	}
}

// loadEnrichedTransaction fetches a transaction by ID and best-effort loads its entries and tags.
func (h *APIHandlers) loadEnrichedTransaction(ctx context.Context, id uuid.UUID) (*models.Transaction, error) {
	txn, err := h.transactions.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	h.enrichTransaction(ctx, txn)
	return txn, nil
}

func (h *APIHandlers) respondTransaction(w http.ResponseWriter, ctx context.Context, txn *models.Transaction) {
	h.enrichTransaction(ctx, txn)
	respondJSON(w, http.StatusOK, transactionToResponse(txn))
}

// APITransactionsList returns transactions with pagination and filters
func (h *APIHandlers) APITransactionsList(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	// Parse query params
	page := queryPage(r)

	perPage := 50
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		perPage, _ = strconv.Atoi(pp)
		if perPage < 1 {
			perPage = 50
		}
		if perPage > 100 {
			perPage = 100
		}
	}

	offset := (page - 1) * perPage

	filter := models.TransactionFilter{
		LedgerID: ledger.ID,
		Limit:    perPage,
		Offset:   offset,
		Search:   r.URL.Query().Get("search"),
	}

	if accID := r.URL.Query().Get("account_id"); accID != "" {
		if id, err := uuid.Parse(accID); err == nil {
			filter.AccountID = &id
		}
	}

	if tagID := r.URL.Query().Get("tag_id"); tagID != "" {
		if id, err := uuid.Parse(tagID); err == nil {
			filter.TagID = &id
		}
	}

	if startDate := r.URL.Query().Get("start_date"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			filter.StartDate = &t
		}
	}

	if endDate := r.URL.Query().Get("end_date"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			filter.EndDate = &t
		}
	}

	if isTransfer := r.URL.Query().Get("is_transfer"); isTransfer != "" {
		b := isTransfer == "true"
		filter.IsTransfer = &b
	}

	transactions, total, err := h.transactions.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, txn := range transactions {
		h.enrichTransaction(r.Context(), txn)
	}

	// Convert to response format
	result := make([]TransactionResponse, len(transactions))
	for i, txn := range transactions {
		result[i] = transactionToResponse(txn)
	}

	totalPages := (total + perPage - 1) / perPage

	respondJSON(w, http.StatusOK, map[string]any{
		"data": result,
		"pagination": APIPagination{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

type createEntryRequest struct {
	AccountID   uuid.UUID `json:"account_id"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency,omitempty"`
}

type createTransactionRequest struct {
	Date        string               `json:"date"`
	Description string               `json:"description"`
	Notes       string               `json:"notes,omitempty"`
	IsTransfer  bool                 `json:"is_transfer,omitempty"`
	Entries     []createEntryRequest `json:"entries"`
}

// APITransactionsCreate creates a new transaction with entries
func (h *APIHandlers) APITransactionsCreate(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	var req createTransactionRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Description == "" {
		respondError(w, http.StatusBadRequest, "description is required")
		return
	}

	if len(req.Entries) < 2 {
		respondError(w, http.StatusBadRequest, "at least 2 entries required for double-entry accounting")
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		date = time.Now()
	}

	// Validate entries sum to zero
	var sum int64
	for _, e := range req.Entries {
		sum += e.AmountCents
	}
	if sum != 0 {
		respondError(w, http.StatusBadRequest, "entries must sum to zero (double-entry accounting)")
		return
	}

	// Verify all accounts belong to this ledger
	for _, e := range req.Entries {
		acc, err := h.accounts.GetByID(r.Context(), e.AccountID)
		if err != nil || acc.LedgerID != ledger.ID {
			respondError(w, http.StatusBadRequest, "invalid account_id: "+e.AccountID.String())
			return
		}
	}

	txn := &models.Transaction{
		LedgerID:    ledger.ID,
		Date:        date,
		Description: req.Description,
		Notes:       req.Notes,
		IsTransfer:  req.IsTransfer,
	}

	entries := make([]*models.Entry, len(req.Entries))
	for i, e := range req.Entries {
		currency := e.Currency
		if currency == "" {
			currency = "USD"
		}
		entries[i] = &models.Entry{
			AccountID:   e.AccountID,
			AmountCents: e.AmountCents,
			Currency:    currency,
		}
	}

	if err := h.transactions.CreateWithEntries(r.Context(), txn, entries); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Reload with entries
	if err := h.transactions.LoadEntries(r.Context(), txn); err != nil {
		slog.WarnContext(r.Context(), "failed to load entries", "txn_id", txn.ID, "err", err)
	}

	respondJSON(w, http.StatusCreated, transactionToResponse(txn))
}

// APITransactionsGet returns a single transaction by ID
func (h *APIHandlers) APITransactionsGet(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	txnID, ok := mustAPIParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	txn, ok := h.getOwnedTransaction(w, r, txnID, ledger.ID)
	if !ok {
		return
	}

	h.respondTransaction(w, r.Context(), txn)
}

type updateTransactionRequest struct {
	Date        *string `json:"date,omitempty"`
	Description *string `json:"description,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	IsTransfer  *bool   `json:"is_transfer,omitempty"`
}

// APITransactionsUpdate updates a transaction
func (h *APIHandlers) APITransactionsUpdate(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	txnID, ok := mustAPIParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	txn, ok := h.getOwnedTransaction(w, r, txnID, ledger.ID)
	if !ok {
		return
	}

	var req updateTransactionRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Date != nil {
		if t, err := time.Parse("2006-01-02", *req.Date); err == nil {
			txn.Date = t
		}
	}
	if req.Description != nil {
		txn.Description = *req.Description
	}
	if req.Notes != nil {
		txn.Notes = *req.Notes
	}
	if req.IsTransfer != nil {
		txn.IsTransfer = *req.IsTransfer
	}

	if err := h.transactions.Update(r.Context(), txn); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondTransaction(w, r.Context(), txn)
}

// APITransactionsDelete deletes a transaction
func (h *APIHandlers) APITransactionsDelete(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	txnID, ok := mustAPIParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	txn, ok := h.getOwnedTransaction(w, r, txnID, ledger.ID)
	if !ok {
		return
	}

	if err := h.transactions.Delete(r.Context(), txn.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondDeleted(w)
}

type addTagRequest struct {
	TagID uuid.UUID `json:"tag_id"`
}

// APITransactionsAddTag adds a tag to a transaction
func (h *APIHandlers) APITransactionsAddTag(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	txnID, ok := mustAPIParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	txn, ok := h.getOwnedTransaction(w, r, txnID, ledger.ID)
	if !ok {
		return
	}

	var req addTagRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Verify tag belongs to this ledger
	tag, err := h.tags.GetByID(r.Context(), req.TagID)
	if err != nil || tag.LedgerID != ledger.ID {
		respondError(w, http.StatusBadRequest, "invalid tag_id")
		return
	}

	// Use CategorizeTransaction to both add the tag and update the entry account
	if err := h.tags.CategorizeTransaction(r.Context(), txnID, req.TagID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondTransaction(w, r.Context(), txn)
}

// APITransactionsRemoveTag removes a tag from a transaction
func (h *APIHandlers) APITransactionsRemoveTag(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	txnID, ok := mustAPIParamUUID(w, r, "id", "transaction ID")
	if !ok {
		return
	}

	txn, ok := h.getOwnedTransaction(w, r, txnID, ledger.ID)
	if !ok {
		return
	}

	tagID, ok := mustAPIParamUUID(w, r, "tagId", "tag ID")
	if !ok {
		return
	}

	if err := h.tags.RemoveTagFromTransaction(r.Context(), txn.ID, tagID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondTransaction(w, r.Context(), txn)
}

func (h *APIHandlers) getOwnedTransaction(w http.ResponseWriter, r *http.Request, id, ledgerID uuid.UUID) (*models.Transaction, bool) {
	txn, err := h.transactions.GetByID(r.Context(), id)
	if err != nil || txn.LedgerID != ledgerID {
		respondError(w, http.StatusNotFound, "transaction not found")
		return nil, false
	}
	return txn, true
}
