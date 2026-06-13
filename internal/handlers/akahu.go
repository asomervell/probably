package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/asomervell/probably/internal/sync"
	"github.com/asomervell/probably/internal/sync/providers"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

const (
	akahuOAuthURL = "https://oauth.akahu.io"
	akahuAPIURL   = "https://api.akahu.io/v1"
)

func (hdl *Handlers) newAkahuSyncService() (*sync.AkahuSyncService, error) {
	client, err := sync.NewAkahuClient(hdl.cfg)
	if err != nil {
		return nil, err
	}
	return sync.NewAkahuSyncService(hdl.db.Pool, client, hdl.cfg), nil
}

func captureAkahuError(ctx context.Context, err error, msg string, opts observability.FailureOptions, logArgs ...any) {
	captureProviderError(ctx, err, msg, opts, sync.IsAkahuTransientError, logArgs...)
}

// AkahuConnect renders the Akahu connection page
// For personal apps with AKAHU_USER_TOKEN configured, it directly syncs accounts
// For full apps, it redirects to Akahu's OAuth authorization endpoint
func (hdl *Handlers) AkahuConnect(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if Akahu is configured
	if hdl.cfg.AkahuAppID == "" {
		page := layouts.AppLayout("Connect Bank - Akahu", user.Email, user.ID.String(),
			shadcn.PageHeader("Connect Your NZ Bank", "Securely link your New Zealand bank account"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.P(h.Class("text-muted-foreground text-center"), g.Text("Akahu is not configured. Please add your Akahu App ID to the environment.")),
				),
			),
		)
		renderHTML(w, page)
		return
	}

	// Check if this is a personal app setup (user token configured)
	if hdl.cfg.AkahuUserToken != "" {
		// Personal app flow - directly sync using the configured user token
		hdl.akahuPersonalAppConnect(w, r, ledger.ID)
		return
	}

	// Full app OAuth flow
	redirectURI := hdl.cfg.BaseURL + "/connections/callback/akahu"
	state := ledger.ID.String() // Use ledger ID as state for CSRF protection

	oauthURL := akahuOAuthURL + "?" + url.Values{
		"response_type": {"code"},
		"client_id":     {hdl.cfg.AkahuAppID},
		"redirect_uri":  {redirectURI},
		"scope":         {"ENDURING_CONSENT ACCOUNTS TRANSACTIONS"},
		"state":         {state},
	}.Encode()

	// Render page with connect button
	page := layouts.AppLayout("Connect Bank - Akahu", user.Email, user.ID.String(),
		shadcn.PageHeader("Connect Your NZ Bank", "Securely link your New Zealand bank account using Akahu"),

		shadcn.Card(shadcn.CardProps{},
			shadcn.CardContentFull(
				h.Div(
					h.ID("akahu-connect"),
					h.Class("flex flex-col items-center justify-center py-12"),

					// Akahu logo placeholder
					h.Div(
						h.Class("mb-6"),
						h.Img(
							g.Attr("src", "https://www.akahu.nz/images/logo-dark.svg"),
							g.Attr("alt", "Akahu"),
							h.Class("h-8"),
						),
					),

					h.P(h.Class("text-muted-foreground text-center mb-6 max-w-md"),
						g.Text("Connect your New Zealand bank accounts securely through Akahu. You'll be redirected to Akahu to authorize access."),
					),

					h.A(
						h.Href(oauthURL),
						shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
							g.Text("Connect with Akahu"),
						),
					),
				),
			),
		),

		// Info section
		h.Div(
			h.Class("mt-6"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.H3(h.Class("font-medium text-foreground mb-2"), g.Text("About Akahu")),
					h.P(h.Class("text-sm text-muted-foreground mb-4"),
						g.Text("Akahu is New Zealand's open finance platform, providing secure access to your bank accounts. Your credentials are never shared with this application."),
					),
					h.Ul(
						h.Class("text-sm text-muted-foreground space-y-2"),
						h.Li(g.Text("• Supports all major NZ banks")),
						h.Li(g.Text("• Bank-level security")),
						h.Li(g.Text("• Read-only access to transactions")),
						h.Li(g.Text("• Disconnect anytime from settings")),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

// akahuPersonalAppConnect handles connection for personal apps with static user token
func (hdl *Handlers) akahuPersonalAppConnect(w http.ResponseWriter, r *http.Request, ledgerID uuid.UUID) {
	slog.InfoContext(r.Context(), "Starting personal app connection for ledger", "ledger_id", ledgerID)

	akahuClient, err := sync.NewAkahuClient(hdl.cfg)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to create Akahu client", "err", err)
		http.Error(w, "Failed to create Akahu client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userToken := hdl.cfg.AkahuUserToken
	if _, err := akahuClient.GetUser(r.Context(), userToken); err != nil {
		slog.ErrorContext(r.Context(), "Failed to validate user token", "err", err)
		http.Error(w, "Invalid Akahu user token: "+err.Error(), http.StatusUnauthorized)
		return
	}
	slog.InfoContext(r.Context(), "Verified Akahu user token")

	hdl.finishAkahuConnect(w, r, ledgerID, akahuClient, userToken)
}

// AkahuCallback handles the OAuth callback from Akahu
func (hdl *Handlers) AkahuCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")

	if errorParam != "" {
		slog.ErrorContext(r.Context(), "OAuth error", "error", errorParam, "description", errorDesc)
		http.Error(w, "Authorization failed: "+errorDesc, http.StatusBadRequest)
		return
	}
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	ledgerID, err := uuid.Parse(state)
	if err != nil {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil || ledger.ID != ledgerID {
		http.Error(w, "Invalid ledger", http.StatusBadRequest)
		return
	}

	akahuClient, err := sync.NewAkahuClient(hdl.cfg)
	if err != nil {
		http.Error(w, "Failed to create Akahu client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	redirectURI := hdl.cfg.BaseURL + "/connections/callback/akahu"
	tokenResp, err := akahuClient.ExchangeAuthCode(r.Context(), code, redirectURI)
	if err != nil {
		slog.ErrorContext(r.Context(), "Token exchange failed", "err", err)
		http.Error(w, "Failed to exchange authorization code: "+err.Error(), http.StatusInternalServerError)
		return
	}

	hdl.finishAkahuConnect(w, r, ledger.ID, akahuClient, tokenResp.AccessToken)
}

// finishAkahuConnect syncs accounts and transactions after obtaining an Akahu access token,
// then redirects to /accounts. Uses a background context so request cancellation does not
// abort the sync mid-flight.
func (hdl *Handlers) finishAkahuConnect(w http.ResponseWriter, r *http.Request, ledgerID uuid.UUID, akahuClient *sync.AkahuClient, accessToken string) {
	syncCtx := context.Background()
	syncService := sync.NewAkahuSyncService(hdl.db.Pool, akahuClient, hdl.cfg)

	accounts, err := syncService.SyncAccounts(syncCtx, ledgerID, accessToken)
	if err != nil {
		slog.ErrorContext(r.Context(), "Account sync failed", "err", err)
		http.Error(w, "Failed to sync accounts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "synced accounts for ledger", "count", len(accounts), "ledger_id", ledgerID)

	var totalTransactions int
	for _, acc := range accounts {
		synced, err := syncService.SyncTransactions(syncCtx, acc)
		if err != nil {
			captureAkahuError(r.Context(), err, "failed to sync transactions after Akahu connect",
				observability.FailureOptions{Component: "akahu_connect", Operation: "sync_transactions", Tags: map[string]string{"account_id": acc.ID.String()}},
				"account_id", acc.ID,
			)
			continue
		}
		totalTransactions += synced
		slog.DebugContext(r.Context(), "synced transactions for account", "count", synced, "account_name", acc.Name)
	}

	slog.InfoContext(r.Context(), "connection complete", "accounts", len(accounts), "transactions", totalTransactions)
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

// AkahuSync manually triggers a sync for Akahu accounts
func (hdl *Handlers) AkahuSync(w http.ResponseWriter, r *http.Request) {
	connectionID := chi.URLParam(r, "connectionId")
	if connectionID == "" {
		connectionID = r.FormValue("connection_id")
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find all Akahu accounts with this connection ID (or all Akahu accounts if no ID provided)
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter to Akahu accounts
	var akahuAccounts []*models.Account
	for _, acc := range accounts {
		if acc.Provider == "akahu" {
			if connectionID == "" || acc.ConnectionID == connectionID {
				akahuAccounts = append(akahuAccounts, acc)
			}
		}
	}

	if len(akahuAccounts) == 0 {
		http.Error(w, "No Akahu accounts found", http.StatusNotFound)
		return
	}

	syncService, err := hdl.newAkahuSyncService()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	syncCtx := context.Background()

	// First, re-sync account metadata
	if len(akahuAccounts) > 0 && akahuAccounts[0].AccessToken != "" {
		_, err = syncService.SyncAccounts(syncCtx, ledger.ID, akahuAccounts[0].AccessToken)
		if err != nil {
			slog.ErrorContext(r.Context(), "Account metadata sync failed", "err", err)
			if providers.IsConnectionDisconnectedError(err) {
				// Clear credentials for all accounts with this connection
				hdl.clearAkahuConnectionCredentials(r.Context(), connectionID)
				http.Redirect(w, r, "/settings/banks?error=connection_revoked", http.StatusSeeOther)
				return
			}
		}
	}

	// Sync transactions for each account
	var totalSynced int
	for _, acc := range akahuAccounts {
		synced, err := syncService.SyncTransactions(syncCtx, acc)
		if err != nil {
			if providers.IsConnectionDisconnectedError(err) {
				hdl.clearAkahuConnectionCredentials(r.Context(), acc.ConnectionID)
				http.Redirect(w, r, "/settings/banks?error=connection_revoked", http.StatusSeeOther)
				return
			}
			captureAkahuError(r.Context(), err, "transaction sync failed",
				observability.FailureOptions{Component: "akahu_sync", Operation: "sync_transactions", Tags: map[string]string{"account_id": acc.ID.String()}},
				"account_id", acc.ID,
			)
			continue
		}
		totalSynced += synced
	}

	slog.InfoContext(r.Context(), "synced total transactions for accounts", "transactions", totalSynced, "accounts", len(akahuAccounts))

	// Redirect back
	redirectURL := "/settings/banks"
	if referer := r.Header.Get("Referer"); referer != "" && strings.Contains(referer, "/accounts") {
		redirectURL = "/accounts"
	}

	htmxRedirect(w, r, redirectURL)
}

// AkahuDisconnect disconnects an Akahu connection
func (hdl *Handlers) AkahuDisconnect(w http.ResponseWriter, r *http.Request) {
	connectionID := chi.URLParam(r, "connectionId")
	if connectionID == "" {
		http.Error(w, "Missing connection ID", http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find accounts with this connection
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Revoke token if possible
	var accessToken string
	for _, acc := range accounts {
		if acc.Provider == "akahu" && acc.ConnectionID == connectionID && acc.AccessToken != "" {
			accessToken = acc.AccessToken
			break
		}
	}

	if accessToken != "" {
		akahuClient, err := sync.NewAkahuClient(hdl.cfg)
		if err == nil {
			if err := akahuClient.RevokeToken(r.Context(), accessToken); err != nil {
				slog.ErrorContext(r.Context(), "Failed to revoke token", "err", err)
				// Continue anyway - we'll still clear local credentials
			}
		}
	}

	// Clear credentials for all accounts with this connection
	hdl.clearAkahuConnectionCredentials(r.Context(), connectionID)

	slog.WarnContext(r.Context(), "Disconnected connection", "id", connectionID)

	// Redirect back
	htmxRedirect(w, r, "/settings/banks")
}

// clearAkahuConnectionCredentials clears credentials for all Akahu accounts with the given connection ID.
// ExternalAccountID is intentionally preserved so accounts can be matched on reconnect.
func (hdl *Handlers) clearAkahuConnectionCredentials(ctx context.Context, connectionID string) {
	accounts, err := hdl.accounts.GetByConnectionID(ctx, connectionID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find accounts for Akahu connection", "connection_id", connectionID, "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "akahu",
			Operation: "clear_credentials_lookup",
		})
		return
	}
	for _, acc := range accounts {
		acc.AccessToken = ""
		acc.ConnectionID = ""
		if err := hdl.accounts.Update(ctx, acc); err != nil {
			slog.ErrorContext(ctx, "failed to clear Akahu credentials for account", "id", acc.ID, "err", err)
		}
	}
}

// AkahuWebhook handles webhook events from Akahu
func (hdl *Handlers) AkahuWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify webhook signature
	signature := r.Header.Get("X-Akahu-Signature")
	if signature == "" {
		http.Error(w, "Missing signature", http.StatusUnauthorized)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature
	if hdl.cfg.AkahuWebhookSecret != "" {
		if !verifyAkahuSignature(body, signature, hdl.cfg.AkahuWebhookSecret) {
			slog.WarnContext(r.Context(), "Invalid signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse webhook payload
	var webhook AkahuWebhookPayload
	if err := json.Unmarshal(body, &webhook); err != nil {
		slog.ErrorContext(r.Context(), "Failed to parse payload", "err", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	slog.InfoContext(r.Context(), "received akahu webhook event", "type", webhook.Type)

	// Acknowledge receipt immediately — Akahu expects a fast 200 and will retry on timeout.
	// Both TOKEN:DELETE and ACCOUNT:UPDATE may involve DB writes or API calls, so they run
	// in background goroutines to avoid blocking the response.
	w.WriteHeader(http.StatusOK)

	switch webhook.Type {
	case "TOKEN:DELETE":
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			defer observability.RecoverAndLog(ctx, "akahu_webhook_token_delete")
			hdl.handleAkahuTokenDelete(ctx, webhook)
		}()

	case "ACCOUNT:UPDATE":
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			defer observability.RecoverAndLog(ctx, "akahu_webhook_account_update")
			hdl.handleAkahuAccountUpdate(ctx, webhook)
		}()

	default:
		slog.InfoContext(r.Context(), "unhandled akahu event type", "type", webhook.Type)
	}
}

// AkahuWebhookPayload represents an Akahu webhook event
type AkahuWebhookPayload struct {
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Item      struct {
		ID string `json:"_id"`
	} `json:"item,omitempty"`
	Token struct {
		ID string `json:"_id"`
	} `json:"token,omitempty"`
	Account struct {
		ID string `json:"_id"`
	} `json:"account,omitempty"`
}

// handleAkahuTokenDelete handles TOKEN:DELETE webhook events.
// Akahu's token ID does not directly map to our stored access token, so we look
// up by connection ID (Item._id) which is stored as ConnectionID on accounts.
func (hdl *Handlers) handleAkahuTokenDelete(ctx context.Context, webhook AkahuWebhookPayload) {
	// Prefer Item._id (connection/item identifier) over Token._id when available,
	// as ConnectionID is what we store on accounts.
	connectionID := webhook.Item.ID
	if connectionID == "" {
		connectionID = webhook.Token.ID
	}

	slog.WarnContext(ctx, "akahu token deleted, clearing credentials", "connection_id", connectionID)

	if connectionID == "" {
		slog.WarnContext(ctx, "akahu TOKEN:DELETE webhook missing both item and token IDs; cannot clear credentials")
		return
	}

	hdl.clearAkahuConnectionCredentials(ctx, connectionID)
}

// handleAkahuAccountUpdate handles ACCOUNT:UPDATE webhook events
func (hdl *Handlers) handleAkahuAccountUpdate(ctx context.Context, webhook AkahuWebhookPayload) {
	accountID := webhook.Account.ID
	if accountID == "" {
		return
	}

	slog.InfoContext(ctx, "Processing ACCOUNT:UPDATE for account", "id", accountID)

	// Find the account by its external (Akahu) account ID.
	// pgx.ErrNoRows is expected: Akahu sends webhooks for all accounts including those
	// not yet imported or already deleted. Treat as a warning, not a failure.
	targetAccount, err := hdl.accounts.GetByExternalAccountID(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		slog.WarnContext(ctx, "akahu webhook: account not found, skipping", "akahu_account_id", accountID)
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "akahu webhook: failed to look up account", "akahu_account_id", accountID, "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "akahu_webhook",
			Operation: "account_update_lookup",
		})
		return
	}

	// Trigger a sync for this account
	syncService, err := hdl.newAkahuSyncService()
	if err != nil {
		slog.ErrorContext(ctx, "Error creating Akahu client", "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "akahu_webhook",
			Operation: "akahu_client_init",
		})
		return
	}
	synced, err := syncService.SyncTransactions(ctx, targetAccount)
	if err != nil {
		slog.ErrorContext(ctx, "Error syncing transactions", "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "akahu_webhook",
			Operation: "transactions_sync",
			Tags:      map[string]string{"account_id": targetAccount.ID.String()},
		})
		return
	}

	slog.InfoContext(ctx, "synced transactions for account", "count", synced, "account_name", targetAccount.Name)
}

// verifyAkahuSignature verifies the webhook signature
func verifyAkahuSignature(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}
