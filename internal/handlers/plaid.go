package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/sync/providers"
)

// PlaidWebhook handles webhook events from Plaid
// POST /api/plaid/webhook
func (hdl *Handlers) PlaidWebhook(w http.ResponseWriter, r *http.Request) {
	// Read the raw body for signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to read body", "err", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify webhook signature when verification is enabled.
	// Plaid signs webhooks with RS256; the public keys are fetched from their JWKS endpoint.
	verificationHeader := r.Header.Get("Plaid-Verification")
	if hdl.cfg.PlaidWebhookSecret != "" {
		if verificationHeader == "" {
			slog.WarnContext(r.Context(), "missing Plaid-Verification header")
			http.Error(w, "Missing verification header", http.StatusUnauthorized)
			return
		}
		if err := hdl.verifyPlaidWebhookJWT(r.Context(), verificationHeader, body); err != nil {
			slog.WarnContext(r.Context(), "Plaid JWT verification failed", "err", err)
			http.Error(w, "Invalid verification token", http.StatusUnauthorized)
			return
		}
	}

	// Parse the webhook payload
	var webhook PlaidWebhookPayload
	if err := json.Unmarshal(body, &webhook); err != nil {
		slog.ErrorContext(r.Context(), "failed to parse webhook", "err", err)
		http.Error(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	slog.InfoContext(r.Context(), "received plaid webhook", "webhook_type", webhook.WebhookType, "webhook_code", webhook.WebhookCode, "item_id", webhook.ItemID)

	// Handle different webhook types
	switch webhook.WebhookType {
	case "TRANSACTIONS":
		hdl.handlePlaidTransactionsWebhook(r.Context(), webhook)

	case "ITEM":
		hdl.handlePlaidItemWebhook(r.Context(), webhook)

	case "AUTH":
		hdl.handlePlaidAuthWebhook(r.Context(), webhook)

	case "INCOME":
		hdl.handlePlaidIncomeWebhook(r.Context(), webhook)

	case "ASSETS":
		hdl.handlePlaidAssetsWebhook(r.Context(), webhook)

	default:
		slog.InfoContext(r.Context(), "unhandled webhook type", "type", webhook.WebhookType)
	}

	// Always respond with 200 OK to acknowledge receipt
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// PlaidWebhookPayload represents a Plaid webhook event
type PlaidWebhookPayload struct {
	WebhookType     string             `json:"webhook_type"`
	WebhookCode     string             `json:"webhook_code"`
	ItemID          string             `json:"item_id"`
	AccountID       string             `json:"account_id,omitempty"`
	NewTransactions int                `json:"new_transactions,omitempty"`
	Error           *PlaidWebhookError `json:"error,omitempty"`
}

// PlaidWebhookError represents an error in a Plaid webhook
type PlaidWebhookError struct {
	ErrorType    string `json:"error_type"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// handlePlaidTransactionsWebhook handles TRANSACTIONS webhook events
func (hdl *Handlers) handlePlaidTransactionsWebhook(ctx context.Context, webhook PlaidWebhookPayload) {
	slog.InfoContext(ctx, "processing TRANSACTIONS webhook", "code", webhook.WebhookCode, "item_id", webhook.ItemID, "account_id", webhook.AccountID, "new_transactions", webhook.NewTransactions)

	switch webhook.WebhookCode {
	case "INITIAL_UPDATE":
		// Initial transaction sync completed
		slog.InfoContext(ctx, "initial transaction update for item", "item_id", webhook.ItemID)
		hdl.triggerPlaidSync(ctx, webhook.ItemID, webhook.AccountID)

	case "HISTORICAL_UPDATE":
		// Historical transaction sync completed
		slog.InfoContext(ctx, "historical transaction update for item", "item_id", webhook.ItemID)
		hdl.triggerPlaidSync(ctx, webhook.ItemID, webhook.AccountID)

	case "DEFAULT_UPDATE":
		// New transactions available
		slog.InfoContext(ctx, "new transactions available", "item_id", webhook.ItemID, "count", webhook.NewTransactions)
		hdl.triggerPlaidSync(ctx, webhook.ItemID, webhook.AccountID)

	case "TRANSACTIONS_REMOVED":
		// Transactions were removed (e.g., pending transaction was cancelled)
		slog.InfoContext(ctx, "transactions removed for item", "item_id", webhook.ItemID)
		// TODO: Handle transaction removal

	default:
		slog.InfoContext(ctx, "unhandled TRANSACTIONS webhook code", "code", webhook.WebhookCode)
	}
}

// handlePlaidItemWebhook handles ITEM webhook events
func (hdl *Handlers) handlePlaidItemWebhook(ctx context.Context, webhook PlaidWebhookPayload) {
	slog.InfoContext(ctx, "processing ITEM webhook", "code", webhook.WebhookCode, "item_id", webhook.ItemID)

	switch webhook.WebhookCode {
	case "ERROR":
		// Item error occurred
		if webhook.Error != nil {
			slog.ErrorContext(ctx, "plaid item error", "error_type", webhook.Error.ErrorType, "error_code", webhook.Error.ErrorCode, "error_message", webhook.Error.ErrorMessage)

			// Check if this is an ITEM_LOGIN_REQUIRED error
			if webhook.Error.ErrorCode == "ITEM_LOGIN_REQUIRED" {
				slog.InfoContext(ctx, "item requires login, marking for update mode", "item_id", webhook.ItemID)
				hdl.markConnectionForUpdateMode(ctx, webhook.ItemID, "login_required")
			}
		}

	case "WEBHOOK_UPDATE_ACKNOWLEDGED":
		// Webhook URL was updated
		slog.InfoContext(ctx, "webhook URL update acknowledged for item", "item_id", webhook.ItemID)

	case "PENDING_EXPIRATION":
		// Item will expire soon (credentials need to be updated)
		slog.InfoContext(ctx, "item pending expiration", "item_id", webhook.ItemID)
		hdl.markConnectionForUpdateMode(ctx, webhook.ItemID, "pending_expiration")

	case "PENDING_DISCONNECT":
		// Item will be disconnected
		slog.InfoContext(ctx, "item pending disconnect", "item_id", webhook.ItemID)
		hdl.markConnectionForUpdateMode(ctx, webhook.ItemID, "pending_disconnect")

	case "LOGIN_REPAIRED":
		// Item login was successfully repaired
		slog.InfoContext(ctx, "item login repaired", "item_id", webhook.ItemID)
		hdl.clearConnectionUpdateMode(ctx, webhook.ItemID)

	case "NEW_ACCOUNTS_AVAILABLE":
		// New accounts are available for this item
		slog.InfoContext(ctx, "new accounts available for item", "item_id", webhook.ItemID)
		hdl.markConnectionForUpdateMode(ctx, webhook.ItemID, "new_accounts_available")

	case "USER_PERMISSION_REVOKED":
		// User revoked access
		slog.InfoContext(ctx, "user permission revoked for item", "item_id", webhook.ItemID)
		hdl.handlePlaidItemDisconnected(ctx, webhook.ItemID)

	default:
		slog.InfoContext(ctx, "unhandled ITEM webhook code", "code", webhook.WebhookCode)
	}
}

// handlePlaidAuthWebhook handles AUTH webhook events
func (hdl *Handlers) handlePlaidAuthWebhook(ctx context.Context, webhook PlaidWebhookPayload) {
	slog.InfoContext(ctx, "processing AUTH webhook", "code", webhook.WebhookCode, "item_id", webhook.ItemID)

	switch webhook.WebhookCode {
	case "AUTOMATICALLY_VERIFIED":
		// Account was automatically verified
		slog.InfoContext(ctx, "account automatically verified for item", "item_id", webhook.ItemID)

	case "VERIFICATION_EXPIRED":
		// Account verification expired
		slog.InfoContext(ctx, "account verification expired for item", "item_id", webhook.ItemID)

	default:
		slog.InfoContext(ctx, "unhandled AUTH webhook code", "code", webhook.WebhookCode)
	}
}

// handlePlaidIncomeWebhook handles INCOME webhook events
func (hdl *Handlers) handlePlaidIncomeWebhook(ctx context.Context, webhook PlaidWebhookPayload) {
	slog.InfoContext(ctx, "processing INCOME webhook", "code", webhook.WebhookCode, "item_id", webhook.ItemID)

	switch webhook.WebhookCode {
	case "PRODUCT_READY":
		// Income data is ready
		slog.InfoContext(ctx, "income data ready for item", "item_id", webhook.ItemID)
		// TODO: Fetch income data

	default:
		slog.InfoContext(ctx, "unhandled INCOME webhook code", "code", webhook.WebhookCode)
	}
}

// handlePlaidAssetsWebhook handles ASSETS webhook events
func (hdl *Handlers) handlePlaidAssetsWebhook(ctx context.Context, webhook PlaidWebhookPayload) {
	slog.InfoContext(ctx, "processing ASSETS webhook", "code", webhook.WebhookCode, "item_id", webhook.ItemID)

	switch webhook.WebhookCode {
	case "PRODUCT_READY":
		// Asset report is ready
		slog.InfoContext(ctx, "asset report ready for item", "item_id", webhook.ItemID)
		// TODO: Fetch asset report

	default:
		slog.InfoContext(ctx, "unhandled ASSETS webhook code", "code", webhook.WebhookCode)
	}
}

// handlePlaidItemDisconnected handles when a Plaid item is disconnected via webhook.
// Uses GetAllWithProviderCredentials (cross-ledger) because webhooks arrive without a ledger ID.
// Keeps ExternalAccountID and ConnectionID for reconnection matching.
func (hdl *Handlers) handlePlaidItemDisconnected(ctx context.Context, itemID string) {
	accounts, err := hdl.accounts.GetAllWithProviderCredentials(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "plaid-webhook: error getting accounts", "err", err)
		return
	}

	for _, acc := range accounts {
		if acc.Provider == "plaid" && acc.ConnectionID == itemID {
			acc.AccessToken = ""
			if err := hdl.accounts.Update(ctx, acc); err != nil {
				slog.ErrorContext(ctx, "plaid-webhook: error clearing credentials", "account_id", acc.ID, "item_id", itemID, "err", err)
			} else {
				slog.InfoContext(ctx, "plaid-webhook: cleared credentials for revoked item", "account_id", acc.ID, "item_id", itemID)
			}
		}
	}
}

// triggerPlaidSync triggers a sync for a Plaid item/account
func (hdl *Handlers) triggerPlaidSync(ctx context.Context, itemID, accountID string) {
	// Find the account by Plaid item ID and account ID
	accounts, err := hdl.accounts.GetAllWithProviderCredentials(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error getting accounts", "err", err)
		return
	}

	var targetAccount *models.Account
	for _, acc := range accounts {
		if acc.Provider == "plaid" && acc.ConnectionID == itemID {
			if accountID == "" || acc.ExternalAccountID == accountID {
				targetAccount = acc
				if accountID != "" {
					break
				}
			}
		}
	}

	if targetAccount == nil {
		slog.WarnContext(ctx, "account not found for plaid item", "item_id", itemID, "account_id", accountID)
		return
	}

	// Sync transactions for the account
	syncService, err := hdl.newPlaidSyncService()
	if err != nil {
		slog.ErrorContext(ctx, "error creating Plaid client", "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "plaid_webhook",
			Operation: "plaid_client_init",
		})
		return
	}
	synced, err := syncService.SyncTransactions(ctx, targetAccount)
	if err != nil {
		if providers.IsConnectionDisconnectedError(err) {
			slog.WarnContext(ctx, "plaid webhook: connection disconnected, marking account", "account_id", targetAccount.ID, "item_id", itemID)
			if dbErr := hdl.accounts.SetConnectionStatus(ctx, targetAccount.ID, "disconnected"); dbErr != nil {
				slog.WarnContext(ctx, "failed to mark account disconnected", "account_id", targetAccount.ID, "err", dbErr)
			}
			return
		}
		slog.ErrorContext(ctx, "error syncing transactions", "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "plaid_webhook",
			Operation: "transactions_sync",
			Tags:      map[string]string{"account_id": targetAccount.ID.String()},
		})
		return
	}
	slog.InfoContext(ctx, "synced transactions for account", "count", synced, "account_id", targetAccount.ID)
}

// markConnectionForUpdateMode marks all accounts with a given connection ID for update mode
func (hdl *Handlers) markConnectionForUpdateMode(ctx context.Context, itemID, reason string) {
	// Find all accounts with this connection ID
	accounts, err := hdl.accounts.GetAllWithProviderCredentials(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error getting accounts", "err", err)
		return
	}

	updated := 0
	for _, acc := range accounts {
		if acc.Provider == "plaid" && acc.ConnectionID == itemID {
			acc.ConnectionStatus = reason
			if err := hdl.accounts.Update(ctx, acc); err != nil {
				slog.ErrorContext(ctx, "error marking account for update mode", "account_id", acc.ID, "err", err)
			} else {
				updated++
				slog.DebugContext(ctx, "marked account for update mode", "account_id", acc.ID, "reason", reason)
			}
		}
	}

	if updated > 0 {
		slog.InfoContext(ctx, "marked accounts for update mode", "count", updated, "item_id", itemID, "reason", reason)
	}
}

// clearConnectionUpdateMode clears update mode status for all accounts with a given connection ID
func (hdl *Handlers) clearConnectionUpdateMode(ctx context.Context, itemID string) {
	// Find all accounts with this connection ID
	accounts, err := hdl.accounts.GetAllWithProviderCredentials(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error getting accounts", "err", err)
		return
	}

	updated := 0
	for _, acc := range accounts {
		if acc.Provider == "plaid" && acc.ConnectionID == itemID && acc.ConnectionStatus != "" {
			acc.ConnectionStatus = ""
			if err := hdl.accounts.Update(ctx, acc); err != nil {
				slog.ErrorContext(ctx, "error clearing update mode for account", "account_id", acc.ID, "err", err)
			} else {
				updated++
				slog.DebugContext(ctx, "cleared update mode for account", "account_id", acc.ID)
			}
		}
	}

	if updated > 0 {
		slog.InfoContext(ctx, "cleared update mode for accounts", "count", updated, "item_id", itemID)
	}
}
