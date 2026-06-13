package handlers

import (
	"net/http"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)


var validAccountTypes = map[models.AccountType]bool{
	models.AccountTypeAsset:     true,
	models.AccountTypeLiability: true,
	models.AccountTypeIncome:    true,
	models.AccountTypeExpense:   true,
	models.AccountTypeEquity:    true,
}

// Account API response types
type AccountResponse struct {
	ID              uuid.UUID          `json:"id"`
	LedgerID        uuid.UUID          `json:"ledger_id"`
	Name            string             `json:"name"`
	Type            models.AccountType `json:"type"`
	InstitutionName string             `json:"institution_name,omitempty"`
	IsActive        bool               `json:"is_active"`
	Balance         int64              `json:"balance"`
	CreatedAt       string             `json:"created_at"`
	UpdatedAt       string             `json:"updated_at"`
}

func accountToResponse(acc *models.Account) AccountResponse {
	return AccountResponse{
		ID:              acc.ID,
		LedgerID:        acc.LedgerID,
		Name:            acc.Name,
		Type:            acc.Type,
		InstitutionName: acc.InstitutionName,
		IsActive:        acc.IsActive,
		Balance:         acc.Balance,
		CreatedAt:       acc.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       acc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// APIAccountsList returns all accounts for the current ledger with balances
func (h *APIHandlers) APIAccountsList(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	accounts, err := h.accounts.GetWithBalances(r.Context(), ledger.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response format
	result := make([]AccountResponse, len(accounts))
	for i, acc := range accounts {
		result[i] = accountToResponse(acc)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}

type createAccountRequest struct {
	Name            string             `json:"name"`
	Type            models.AccountType `json:"type"`
	InstitutionName string             `json:"institution_name,omitempty"`
}

// APIAccountsCreate creates a new account
func (h *APIHandlers) APIAccountsCreate(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	var req createAccountRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.Type == "" {
		respondError(w, http.StatusBadRequest, "type is required")
		return
	}

	// Validate account type
	if !validAccountTypes[req.Type] {
		respondError(w, http.StatusBadRequest, "invalid account type, must be one of: asset, liability, income, expense, equity")
		return
	}

	account := &models.Account{
		LedgerID:        ledger.ID,
		Name:            req.Name,
		Type:            req.Type,
		InstitutionName: req.InstitutionName,
		IsActive:        true,
	}

	if err := h.accounts.Create(r.Context(), account); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, accountToResponse(account))
}

// APIAccountsGet returns a single account by ID
func (h *APIHandlers) APIAccountsGet(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	accountID, ok := mustAPIParamUUID(w, r, "id", "account ID")
	if !ok {
		return
	}

	account, ok := h.getOwnedAccount(w, r, accountID, ledger.ID)
	if !ok {
		return
	}

	// Get the balance
	balance, err := h.accounts.GetBalance(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	account.Balance = balance

	respondJSON(w, http.StatusOK, accountToResponse(account))
}

type updateAccountRequest struct {
	Name            *string             `json:"name,omitempty"`
	Type            *models.AccountType `json:"type,omitempty"`
	InstitutionName *string             `json:"institution_name,omitempty"`
	IsActive        *bool               `json:"is_active,omitempty"`
}

// APIAccountsUpdate updates an account
func (h *APIHandlers) APIAccountsUpdate(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	accountID, ok := mustAPIParamUUID(w, r, "id", "account ID")
	if !ok {
		return
	}

	account, ok := h.getOwnedAccount(w, r, accountID, ledger.ID)
	if !ok {
		return
	}

	var req updateAccountRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name != nil {
		account.Name = *req.Name
	}
	if req.Type != nil {
		// Validate account type
		if !validAccountTypes[*req.Type] {
			respondError(w, http.StatusBadRequest, "invalid account type")
			return
		}
		account.Type = *req.Type
	}
	if req.InstitutionName != nil {
		account.InstitutionName = *req.InstitutionName
	}
	if req.IsActive != nil {
		account.IsActive = *req.IsActive
	}

	if err := h.accounts.Update(r.Context(), account); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get the balance
	balance, err := h.accounts.GetBalance(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	account.Balance = balance

	respondJSON(w, http.StatusOK, accountToResponse(account))
}

// APIAccountsDelete deletes an account
func (h *APIHandlers) APIAccountsDelete(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	accountID, ok := mustAPIParamUUID(w, r, "id", "account ID")
	if !ok {
		return
	}

	if _, ok := h.getOwnedAccount(w, r, accountID, ledger.ID); !ok {
		return
	}

	// Delete account and all its transactions
	deleted, err := h.accounts.DeleteWithTransactions(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"deleted":      true,
		"transactions": deleted,
	})
}

func (h *APIHandlers) getOwnedAccount(w http.ResponseWriter, r *http.Request, id, ledgerID uuid.UUID) (*models.Account, bool) {
	acc, err := h.accounts.GetByID(r.Context(), id)
	if err != nil || acc.LedgerID != ledgerID {
		respondError(w, http.StatusNotFound, "account not found")
		return nil, false
	}
	return acc, true
}
