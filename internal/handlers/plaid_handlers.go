package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/sync"
	"github.com/asomervell/probably/internal/sync/providers"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func (hdl *Handlers) newPlaidSyncService() (*sync.PlaidSyncService, error) {
	client, err := sync.NewPlaidClient(hdl.cfg)
	if err != nil {
		return nil, err
	}
	return sync.NewPlaidSyncService(hdl.db.Pool, client, hdl.cfg), nil
}

func (hdl *Handlers) createLinkTokenWithRedirectFallback(ctx context.Context, plaidClient *sync.PlaidClient, userID, accessToken, redirectURI string, accountSelectionEnabled bool) (string, error) {
	linkToken, err := plaidClient.CreateLinkToken(ctx, userID, accessToken, redirectURI, accountSelectionEnabled)
	if err == nil {
		return linkToken, nil
	}
	if redirectURI != "" && strings.Contains(strings.ToLower(err.Error()), "oauth redirect uri") {
		fallbackToken, fallbackErr := plaidClient.CreateLinkToken(ctx, userID, accessToken, "", accountSelectionEnabled)
		if fallbackErr == nil {
			slog.InfoContext(ctx, "plaid: CreateLinkToken retry without redirect URI succeeded", "original_redirect_uri", redirectURI)
			return fallbackToken, nil
		}
	}
	return "", err
}

// renderPlaidLinkPage renders a page with Plaid Link initialized with the given token
func (hdl *Handlers) renderPlaidLinkPage(w http.ResponseWriter, r *http.Request, user *models.User, linkToken, title, subtitle string) {
	if user == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	// Embed token as base64 + atob so the literal cannot break out of <script> (e.g. </script> in a JWT).
	b64 := base64.StdEncoding.EncodeToString([]byte(linkToken))
	b64JSON, err := json.Marshal(b64)
	if err != nil {
		b64JSON = []byte(`""`)
	}
	plaidInlineScript := `document.addEventListener('DOMContentLoaded', function() {
	var el = document.getElementById('plaid-link');
	function showPlaidError(msg) {
		if (!el) return;
		var t = String(msg);
		t = t.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
		el.innerHTML = '<p class="text-destructive text-sm p-2">' + t + '</p>';
	}
	try {
		if (window.__plaidLinkLoadFailed) {
			showPlaidError('Plaid Link script could not load (network, firewall, or extension blocked cdn.plaid.com).');
			return;
		}
		var linkToken = (function() {
			try { return atob(` + string(b64JSON) + `); } catch (e) { return ''; }
		})();
		if (!linkToken) {
			showPlaidError('Plaid is not configured (missing link token). Check server logs and Plaid credentials.');
			return;
		}
		if (typeof Plaid === 'undefined') {
			showPlaidError('Plaid Link is not available. The script from cdn.plaid.com may be blocked; try another network or disable blockers.');
			return;
		}
		var handler = Plaid.create({
			token: linkToken,
			onSuccess: function(public_token, metadata) {
				document.getElementById('plaid-link').innerHTML = '<div class="text-center"><div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary mb-4"></div><p class="text-foreground font-medium">Syncing your accounts...</p><p class="text-muted-foreground text-sm mt-2">This may take a moment</p></div>';
				window.location.href = '/connections/callback/plaid?public_token=' + encodeURIComponent(public_token) + '&item_id=' + encodeURIComponent(metadata.item_id);
			},
			onExit: function(err, metadata) {
				if (err != null) {
					console.error('Plaid Link error:', err);
					var errorMessage = 'Connection was cancelled or an error occurred.';
					if (err && err.display_message) { errorMessage = err.display_message; }
					else if (err && err.error_message) { errorMessage = err.error_message; }
					if (typeof showToast === 'function') {
						showToast({ variant: 'error', title: 'Connection Error', description: errorMessage, duration: 8000 });
					} else { showPlaidError(errorMessage); }
				} else {
					window.location.href = '/accounts';
				}
			},
			onEvent: function(eventName, metadata) { console.log('Plaid Link event:', eventName, metadata); }
		});
		var btn = document.createElement('button');
		btn.className = 'bg-indigo-600 text-white rounded-lg px-6 py-3 text-sm font-medium hover:bg-indigo-500 transition-colors';
		btn.innerText = 'Connect Bank Account';
		btn.onclick = function() { handler.open(); };
		el.appendChild(btn);
	} catch (e) {
		console.error('Plaid Link init error:', e);
		showPlaidError(e && e.message ? e.message : 'Plaid could not start.');
	}
});`
	page := layouts.AppLayout(title, user.Email, user.ID.String(),
		shadcn.PageHeader(title, subtitle),

		shadcn.Card(shadcn.CardProps{},
			shadcn.CardContentFull(
				h.Div(
					h.ID("plaid-link"),
					h.Class("flex items-center justify-center py-12"),
				),

				// Plaid Link — blocking load so this runs before the next (inline) script; no defer/async.
				h.Script(
					g.Attr("src", "https://cdn.plaid.com/link/v2/stable/link-initialize.js"),
					g.Attr("onerror", "window.__plaidLinkLoadFailed=true"),
				),

				h.Script(
					g.Raw(plaidInlineScript),
				),
			),
		),

		// Info section
		h.Div(
			h.Class("mt-6"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.H3(h.Class("font-medium text-foreground mb-2"), g.Text("About Plaid")),
					h.P(h.Class("text-sm text-muted-foreground mb-4"),
						g.Text("Plaid provides secure, read-only access to your bank accounts. Your credentials are never stored by this application."),
					),
					h.Ul(
						h.Class("text-sm text-muted-foreground space-y-2"),
						h.Li(g.Text("• Bank-level encryption")),
						h.Li(g.Text("• Read-only access to transactions")),
						h.Li(g.Text("• Disconnect anytime from settings")),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

// PlaidConnect renders the Plaid Link connection page
func (hdl *Handlers) PlaidConnect(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	_, err := hdl.getCurrentLedger(r)
	if err != nil {
		slog.ErrorContext(r.Context(), "plaid: PlaidConnect getCurrentLedger", "err", err)
		if strings.Contains(err.Error(), "user person entity not found") {
			http.Error(w, "Account setup is incomplete (no person entity for this user). Fix database seeding or contact support.", http.StatusInternalServerError)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create Plaid client to generate link token
	plaidClient, err := sync.NewPlaidClient(hdl.cfg)
	if err != nil {
		slog.ErrorContext(r.Context(), "plaid: PlaidConnect NewPlaidClient", "err", err)
		http.Error(w, "Plaid is not configured: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create link token
	// Use PLAID_REDIRECT_URI if configured, otherwise don't set redirect URI
	// Redirect URI must be registered in Plaid dashboard if used
	redirectURI := hdl.cfg.PlaidRedirectURI
	// If redirect URI is a path (starts with /), prepend BaseURL to make it a full URI
	if redirectURI != "" && strings.HasPrefix(redirectURI, "/") {
		redirectURI = hdl.cfg.BaseURL + redirectURI
	}
	// Check if account_selection_enabled is requested (for update mode with new accounts)
	accountSelectionEnabled := r.URL.Query().Get("account_selection_enabled") == "true"
	linkToken, err := hdl.createLinkTokenWithRedirectFallback(r.Context(), plaidClient, user.ID.String(), "", redirectURI, accountSelectionEnabled)
	if err != nil {
		slog.ErrorContext(r.Context(), "plaid: PlaidConnect CreateLinkToken failed", "err", err, "redirect_uri", redirectURI, "webhook", hdl.cfg.PlaidWebhookURL, "env", hdl.cfg.PlaidEnvironment)
		http.Error(w, "Failed to initialize Plaid Link: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Render page with Plaid Link embedded
	hdl.renderPlaidLinkPage(w, r, user, linkToken, "Connect Your Bank", "Securely link your bank account using Plaid")
}

// PlaidCallback handles the Plaid OAuth callback
func (hdl *Handlers) PlaidCallback(w http.ResponseWriter, r *http.Request) {
	publicToken := r.URL.Query().Get("public_token")
	itemID := r.URL.Query().Get("item_id")

	if publicToken == "" {
		http.Error(w, "Missing public token", http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create Plaid client
	plaidClient, err := sync.NewPlaidClient(hdl.cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Exchange public token for access token
	accessToken, exchangedItemID, err := plaidClient.ExchangePublicToken(r.Context(), publicToken)
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Use item_id from exchange if not provided in query
	if itemID == "" {
		itemID = exchangedItemID
	}

	// Check for duplicate Item - same Plaid Item ID already connected for this ledger
	existingAccounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err == nil {
		for _, acc := range existingAccounts {
			if acc.Provider == "plaid" && acc.ConnectionID == itemID {
				// Duplicate Item detected - show error and offer to reconnect
				cu := auth.CurrentUser(r)
				phEmail, phID := "", ""
				if cu != nil {
					phEmail, phID = cu.Email, cu.ID.String()
				}
				page := layouts.AppLayout("Connection Error", phEmail, phID,
					shadcn.PageHeader("Connection Already Exists", "This bank connection is already linked to your account"),
					shadcn.Card(shadcn.CardProps{},
						shadcn.CardContentFull(
							h.Div(
								h.Class("space-y-4"),
								h.Div(
									h.Class("bg-ring/10 border border-ring/20 rounded-lg p-4"),
									h.Div(
										h.Class("flex items-start gap-3"),
										h.Div(
											h.Class("text-ring mt-0.5"),
											g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
												<path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
											</svg>`),
										),
										h.Div(
											h.H3(h.Class("font-medium text-ring"), g.Text("Duplicate Connection")),
											h.P(h.Class("text-sm text-ring/80 mt-1"),
												g.Text("This bank connection is already linked to your account. You can reconnect it if needed."),
											),
										),
									),
								),
								h.Div(
									h.Class("flex items-center gap-4 pt-4"),
									h.A(
										h.Href("/connections/reconnect/plaid?connection_id="+itemID),
										h.Class("inline-flex items-center gap-2 bg-primary text-primary-foreground rounded-lg px-4 py-2 text-sm font-medium hover:opacity-90 transition-colors"),
										layouts.IconLink(),
										g.Text("Reconnect Existing Connection"),
									),
									h.A(
										h.Href("/accounts"),
										h.Class("text-muted-foreground hover:text-foreground text-sm"),
										g.Text("Go to Accounts"),
									),
								),
							),
						),
					),
				)
				renderHTML(w, page)
				return
			}
		}
	}

	// Use background context - sync can take a long time
	syncCtx := context.Background()

	// Sync accounts
	syncService := sync.NewPlaidSyncService(hdl.db.Pool, plaidClient, hdl.cfg)
	accounts, err := syncService.SyncAccounts(syncCtx, ledger.ID, accessToken)
	if err != nil {

		// Check if this is a connection disconnection error (needs re-authentication)
		if providers.IsConnectionDisconnectedError(err) {
			redirectURL := "/connections/reconnect/plaid?connection_id=" + itemID + "&message=Your bank connection needs to be re-authenticated"
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
			return
		}

		http.Error(w, "Failed to sync accounts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sync transactions for each account
	for _, acc := range accounts {
		synced, err := syncService.SyncTransactions(syncCtx, acc)
		if err != nil {
			slog.ErrorContext(syncCtx, "plaid-callback: error syncing transactions", "account", acc.Name, "err", err)
			// Continue with other accounts even if one fails
			continue
		}
		slog.DebugContext(syncCtx, "plaid-callback: synced transactions", "account", acc.Name, "count", synced)
	}

	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

// PlaidReconnect shows a page explaining that the bank connection needs re-authentication
func (hdl *Handlers) PlaidReconnect(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	accountID := r.URL.Query().Get("account_id")
	connectionIDParam := r.URL.Query().Get("connection_id")
	message := r.URL.Query().Get("message")
	accountSelectionEnabled := r.URL.Query().Get("account_selection_enabled") == "true"

	// If connection_id is provided, we want to initialize Plaid Link in update mode
	if connectionIDParam != "" {
		ledger, err := hdl.getCurrentLedger(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Find an account with this connection ID to get the access token
		accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var accessToken string
		for _, acc := range accounts {
			if acc.Provider == "plaid" && acc.ConnectionID == connectionIDParam && acc.AccessToken != "" {
				accessToken = acc.AccessToken
				break
			}
		}

		if accessToken != "" {
			// Create Plaid client
			plaidClient, err := sync.NewPlaidClient(hdl.cfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Create link token in update mode
			redirectURI := hdl.cfg.PlaidRedirectURI
			if redirectURI != "" && strings.HasPrefix(redirectURI, "/") {
				redirectURI = hdl.cfg.BaseURL + redirectURI
			}

			linkToken, err := hdl.createLinkTokenWithRedirectFallback(r.Context(), plaidClient, user.ID.String(), accessToken, redirectURI, accountSelectionEnabled)
			if err != nil {
				// If token creation fails, fall back to showing the explanation page
				slog.WarnContext(r.Context(), "plaid-reconnect: failed to create update mode link token", "err", err)
			} else {
				// We have a link token! Render the connection page with this token
				// This uses the same UI as PlaidConnect but with the update mode token
				hdl.renderPlaidLinkPage(w, r, user, linkToken, "Update Bank Connection", "Securely re-authenticate your bank connection")
				return
			}
		}
	}

	// Fallback: show the explanation page if no connection_id or token creation failed
	var reconnectURL string
	var accountName string
	if accountID != "" {
		reconnectURL = "/connections/link/plaid/" + accountID
		// Try to get the account name for a better message
		if accUUID, err := uuid.Parse(accountID); err == nil {
			if acc, err := hdl.accounts.GetByID(r.Context(), accUUID); err == nil {
				accountName = acc.Name
			}
		}
	} else if connectionIDParam != "" {
		// If connection ID provided but no account ID, use reconnect endpoint
		reconnectURL = "/connections/reconnect/plaid?connection_id=" + connectionIDParam
	} else {
		reconnectURL = "/connections/connect/plaid"
	}

	// If account_selection_enabled is requested, add it to the URL
	if accountSelectionEnabled {
		if strings.Contains(reconnectURL, "?") {
			reconnectURL += "&account_selection_enabled=true"
		} else {
			reconnectURL += "?account_selection_enabled=true"
		}
	}

	// Use custom message if provided, otherwise use default
	headerMessage := "Your bank requires re-authentication"
	if message != "" {
		headerMessage = message
	}

	page := layouts.AppLayout("Reconnect Bank", user.Email, user.ID.String(),
		shadcn.PageHeader("Bank Reconnection Required", headerMessage),

		// Alert card
		h.Div(
			h.Class("mb-6"),
			h.Div(
				h.Class("bg-ring/10 border border-ring/20 rounded-lg p-4"),
				h.Div(
					h.Class("flex items-start gap-3"),
					h.Div(
						h.Class("text-ring mt-0.5"),
						g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
							<path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
						</svg>`),
					),
					h.Div(
						h.H3(h.Class("font-medium text-ring"), g.Text("Multi-Factor Authentication Required")),
						h.P(h.Class("text-sm text-ring/80 mt-1"),
							g.Text("Your bank is requesting additional verification. This is a normal security measure that banks require periodically."),
						),
					),
				),
			),
		),

		// Main card with explanation and action
		shadcn.Card(shadcn.CardProps{},
			shadcn.CardContentFull(
				g.If(accountName != "",
					h.Div(
						h.Class("mb-4"),
						h.P(h.Class("text-muted-foreground"),
							g.Text("Account: "),
							h.Span(h.Class("text-foreground font-medium"), g.Text(accountName)),
						),
					),
				),

				h.Div(
					h.Class("space-y-4"),
					h.Div(
						h.H3(h.Class("font-medium text-foreground mb-2"), g.Text("What happened?")),
						h.P(h.Class("text-sm text-muted-foreground"),
							g.Text("Your bank has disconnected the connection and requires you to log in again. This typically happens when:"),
						),
						h.Ul(
							h.Class("text-sm text-muted-foreground mt-2 space-y-1 ml-4"),
							h.Li(g.Text("• Your bank requires periodic re-verification")),
							h.Li(g.Text("• You changed your banking password")),
							h.Li(g.Text("• Your bank's security systems flagged the connection")),
						),
					),

					h.Div(
						h.H3(h.Class("font-medium text-foreground mb-2"), g.Text("How to fix it")),
						h.P(h.Class("text-sm text-muted-foreground"),
							g.Text("Click the button below to securely reconnect. You'll be asked to log in to your bank and may need to complete a verification step (like entering a code sent to your phone)."),
						),
					),

					h.Div(
						h.Class("flex items-center gap-4 pt-4"),
						h.A(
							h.Href(reconnectURL),
							h.Class("inline-flex items-center gap-2 bg-primary text-primary-foreground rounded-lg px-4 py-2 text-sm font-medium hover:opacity-90 transition-colors"),
							layouts.IconLink(),
							g.Text("Reconnect Bank Account"),
						),
						h.A(
							h.Href("/accounts"),
							h.Class("text-muted-foreground hover:text-foreground text-sm"),
							g.Text("Cancel"),
						),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

// PlaidLink renders the Plaid Link page for linking to an existing account
func (hdl *Handlers) PlaidLink(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	accountID := chi.URLParam(r, "accountId")

	// Verify the account exists and belongs to this user's ledger
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	accUUID, err := uuid.Parse(accountID)
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accUUID)
	if err != nil || account.LedgerID != ledger.ID {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Create Plaid client to generate link token
	plaidClient, err := sync.NewPlaidClient(hdl.cfg)
	if err != nil {
		http.Error(w, "Plaid is not configured: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create link token
	// Use PLAID_REDIRECT_URI if configured, append account_id if provided
	// Redirect URI must be registered in Plaid dashboard if used
	redirectURI := hdl.cfg.PlaidRedirectURI
	// If redirect URI is a path (starts with /), prepend BaseURL to make it a full URI
	if redirectURI != "" && strings.HasPrefix(redirectURI, "/") {
		redirectURI = hdl.cfg.BaseURL + redirectURI
	}
	if redirectURI != "" && accountID != "" {
		// Append account_id as query parameter if redirect URI is set
		if strings.Contains(redirectURI, "?") {
			redirectURI += "&account_id=" + accountID
		} else {
			redirectURI += "?account_id=" + accountID
		}
	}
	// Check if account_selection_enabled is requested
	accountSelectionEnabled := r.URL.Query().Get("account_selection_enabled") == "true"
	linkToken, err := hdl.createLinkTokenWithRedirectFallback(r.Context(), plaidClient, user.ID.String(), "", redirectURI, accountSelectionEnabled)
	if err != nil {
		http.Error(w, "Failed to initialize Plaid Link: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Render page with Plaid Link embedded
	hdl.renderPlaidLinkPage(w, r, user, linkToken, "Link Bank Account", fmt.Sprintf("Connect your existing account '%s' to a bank", account.Name))
}

// findConnectionAccessToken returns the access token and first account ID for a Plaid connection.
func findConnectionAccessToken(accounts []*models.Account, connectionID string) (token, accountID string) {
	for _, acc := range accounts {
		if acc.ConnectionID == connectionID && acc.AccessToken != "" {
			if accountID == "" {
				accountID = acc.ID.String()
			}
			token = acc.AccessToken
			break
		}
	}
	return
}

// redirectPlaidDisconnected marks the connection for update mode, clears credentials,
// and issues a redirect to the Plaid reconnect page. The caller must return after this.
func (hdl *Handlers) redirectPlaidDisconnected(w http.ResponseWriter, r *http.Request, ledgerID uuid.UUID, connectionID, accountID string) {
	hdl.markConnectionForUpdateMode(r.Context(), connectionID, "login_required")
	hdl.clearPlaidItemCredentials(r.Context(), ledgerID, connectionID)

	redirectURL := "/connections/reconnect/plaid?connection_id=" + connectionID
	if accountID != "" {
		redirectURL += "&account_id=" + accountID
	}

	htmxRedirect(w, r, redirectURL)
}

// plaidErrorMessage extracts a user-friendly message from a Plaid error.
func plaidErrorMessage(err error) string {
	var plaidErr *sync.PlaidItemError
	if errors.As(err, &plaidErr) {
		return plaidErr.Message
	}
	return err.Error()
}

// PlaidSync syncs accounts for a Plaid item
func (hdl *Handlers) PlaidSync(w http.ResponseWriter, r *http.Request) {
	// Standardized: use connectionId as primary, itemId for backwards compatibility
	connectionID := chi.URLParam(r, "connectionId")
	if connectionID == "" {
		connectionID = chi.URLParam(r, "itemId")
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find accounts with this item ID
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	syncService, err := hdl.newPlaidSyncService()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use background context - sync can take a long time
	syncCtx := context.Background()

	accessToken, firstAccountID := findConnectionAccessToken(accounts, connectionID)
	if accessToken == "" {
		http.Error(w, "No access token found for this connection", http.StatusBadRequest)
		return
	}

	// Sync accounts
	_, err = syncService.SyncAccounts(syncCtx, ledger.ID, accessToken)
	if err != nil {
		slog.ErrorContext(r.Context(), "plaid-sync: error syncing accounts", "err", err)
		if providers.IsConnectionDisconnectedError(err) {
			hdl.redirectPlaidDisconnected(w, r, ledger.ID, connectionID, firstAccountID)
			return
		}
		http.Error(w, "Failed to sync accounts: "+plaidErrorMessage(err), http.StatusInternalServerError)
		return
	}

	// Sync transactions for all accounts with this connection ID
	for _, acc := range accounts {
		if acc.ConnectionID == connectionID {
			synced, err := syncService.SyncTransactions(syncCtx, acc)
			if err != nil {
				if providers.IsConnectionDisconnectedError(err) {
					hdl.redirectPlaidDisconnected(w, r, ledger.ID, connectionID, acc.ID.String())
					return
				}
				slog.ErrorContext(r.Context(), "plaid-sync: error syncing transactions", "account", acc.Name, "err", err)
				continue
			}
			slog.DebugContext(r.Context(), "plaid-sync: synced transactions", "account", acc.Name, "count", synced)
		}
	}

	// HTMX response or redirect
	htmxRedirect(w, r, "/settings/banks")
}

// PlaidFullResync performs a complete resync
func (hdl *Handlers) PlaidFullResync(w http.ResponseWriter, r *http.Request) {
	// Standardized: use connectionId as primary, itemId for backwards compatibility
	connectionID := chi.URLParam(r, "connectionId")
	if connectionID == "" {
		connectionID = chi.URLParam(r, "itemId")
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find accounts with this connection ID
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	syncService, err := hdl.newPlaidSyncService()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use background context
	syncCtx := context.Background()

	accessToken, firstAccountID := findConnectionAccessToken(accounts, connectionID)
	if accessToken == "" {
		http.Error(w, "No access token found for this connection", http.StatusBadRequest)
		return
	}

	// Re-sync account metadata
	_, err = syncService.SyncAccounts(syncCtx, ledger.ID, accessToken)
	if err != nil {
		slog.ErrorContext(r.Context(), "plaid-resync: error syncing accounts", "err", err)
		if providers.IsConnectionDisconnectedError(err) {
			hdl.redirectPlaidDisconnected(w, r, ledger.ID, connectionID, firstAccountID)
			return
		}
		http.Error(w, "Failed to resync accounts: "+plaidErrorMessage(err), http.StatusInternalServerError)
		return
	}

	// Delete and re-import all transactions
	var totalSynced int
	for _, acc := range accounts {
		if acc.ConnectionID == connectionID {
			synced, err := syncService.DeleteAndResync(syncCtx, acc)
			if err != nil && providers.IsConnectionDisconnectedError(err) {
				hdl.redirectPlaidDisconnected(w, r, ledger.ID, connectionID, acc.ID.String())
				return
			}
			totalSynced += synced
		}
	}

	// HTMX response or redirect
	htmxRedirect(w, r, "/settings/banks")
}

// PlaidDisconnect disconnects a Plaid connection
func (hdl *Handlers) PlaidDisconnect(w http.ResponseWriter, r *http.Request) {
	// Standardized: use connectionId as primary, itemId for backwards compatibility
	connectionID := chi.URLParam(r, "connectionId")
	if connectionID == "" {
		connectionID = chi.URLParam(r, "itemId")
	}
	if connectionID == "" {
		http.Error(w, "Missing connection ID", http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find accounts with this connection ID
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Revoke token with Plaid if possible
	var accessToken string
	for _, acc := range accounts {
		if acc.Provider == "plaid" && acc.ConnectionID == connectionID && acc.AccessToken != "" {
			accessToken = acc.AccessToken
			break
		}
	}

	// Collect all account IDs with this connection ID for deletion
	var accountIDsToDelete []uuid.UUID
	for _, acc := range accounts {
		if acc.Provider == "plaid" && acc.ConnectionID == connectionID {
			accountIDsToDelete = append(accountIDsToDelete, acc.ID)
		}
	}

	// Revoke token with Plaid if possible
	if accessToken != "" {
		plaidClient, err := sync.NewPlaidClient(hdl.cfg)
		if err == nil {
			if err := plaidClient.ItemRemove(r.Context(), accessToken); err != nil {
				slog.WarnContext(r.Context(), "plaid-disconnect: failed to revoke token", "err", err)
				// Continue anyway - we'll still delete local data
			} else {
				slog.InfoContext(r.Context(), "plaid-disconnect: removed Plaid item", "connection_id", connectionID)
			}
		}
	}

	// Delete all related data (transactions and accounts) for data retention compliance
	// This ensures all Plaid consumer data is deleted per data retention policies
	totalTransactionsDeleted := int64(0)
	for _, accountID := range accountIDsToDelete {
		deleted, err := hdl.accounts.DeleteWithTransactions(r.Context(), accountID)
		if err != nil {
			slog.ErrorContext(r.Context(), "plaid-disconnect: failed to delete account", "account_id", accountID, "err", err)
			// Continue with other accounts
			continue
		}
		totalTransactionsDeleted += deleted
		slog.DebugContext(r.Context(), "plaid-disconnect: deleted account", "account_id", accountID, "transactions_deleted", deleted)
	}

	slog.InfoContext(r.Context(), "plaid-disconnect: disconnected connection", "connection_id", connectionID, "accounts_deleted", len(accountIDsToDelete), "transactions_deleted", totalTransactionsDeleted)

	// Redirect back to settings/banks after disconnect
	htmxRedirect(w, r, "/settings/banks")
}

// clearPlaidItemCredentials clears credentials for accounts with a Plaid item ID
// Keeps ExternalAccountID and ConnectionID for reconnection matching
func (hdl *Handlers) clearPlaidItemCredentials(ctx context.Context, ledgerID uuid.UUID, itemID string) {
	accounts, err := hdl.accounts.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		slog.ErrorContext(ctx, "plaid-disconnect: error getting accounts for ledger", "ledger_id", ledgerID, "err", err)
		return
	}

	for _, acc := range accounts {
		if acc.Provider == "plaid" && acc.ConnectionID == itemID {
			acc.AccessToken = ""
			// DO NOT clear ExternalAccountID or ConnectionID - these are persistent and needed for matching
			// acc.ExternalAccountID stays (Plaid account ID)
			// acc.ConnectionID stays (Plaid item ID)
			if err := hdl.accounts.Update(ctx, acc); err != nil {
				slog.ErrorContext(ctx, "plaid-disconnect: error clearing credentials", "account_id", acc.ID, "err", err)
			} else {
				slog.DebugContext(ctx, "plaid-disconnect: cleared credentials", "account_id", acc.ID, "account_name", acc.Name, "external_account_id", acc.ExternalAccountID, "item_id", acc.ConnectionID)
			}
		}
	}
}
