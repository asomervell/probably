package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/enrichment"
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

// errNoAccessToken is returned by syncSingleEnrollment when no stored access token
// is found for an enrollment. Callers use errors.Is to detect this case and redirect
// the user to reconnect rather than treating it as a transient API failure.
var errNoAccessToken = errors.New("no access token for enrollment")

// accountBelongsToEnrollment reports whether an account is part of the given enrollment.
func accountBelongsToEnrollment(acc *models.Account, enrollmentID string) bool {
	return acc.ConnectionID == enrollmentID || acc.TellerEnrollmentID == enrollmentID
}

// enrollmentIDFromRequest extracts the enrollment/connection ID from URL parameters,
// trying multiple names for backwards compatibility with older route patterns.
func enrollmentIDFromRequest(r *http.Request) string {
	if id := chi.URLParam(r, "enrollmentId"); id != "" {
		return id
	}
	if id := chi.URLParam(r, "connectionId"); id != "" {
		return id
	}
	return chi.URLParam(r, "id")
}

// captureProviderError logs a provider sync error and, unless isTransient(err) returns true,
// captures it to PostHog. Transient errors (5xx, 429) are warned without capture so the sync
// worker retries them automatically.
func captureProviderError(ctx context.Context, err error, msg string, opts observability.FailureOptions, isTransient func(error) bool, logArgs ...any) {
	if isTransient(err) {
		slog.WarnContext(ctx, msg+" (transient, will retry)", logArgs...)
		return
	}
	args := make([]any, 0, len(logArgs)+2)
	args = append(args, logArgs...)
	args = append(args, "err", err)
	slog.WarnContext(ctx, msg, args...)
	observability.CaptureFailure(ctx, err, opts)
}

func captureTellerError(ctx context.Context, err error, msg string, opts observability.FailureOptions, logArgs ...any) {
	captureProviderError(ctx, err, msg, opts, sync.IsTellerTransientError, logArgs...)
}

func (hdl *Handlers) newTellerSyncService() (*sync.TellerSyncService, error) {
	if hdl.tellerSyncService == nil {
		return nil, fmt.Errorf("Teller client not initialized (check TLS certificate config)")
	}
	return hdl.tellerSyncService, nil
}

func (hdl *Handlers) TellerConnect(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)

	// Render page with Teller Connect embedded
	page := layouts.AppLayout("Connect Bank", user.Email, user.ID.String(),
		shadcn.PageHeader("Connect Your Bank", "Securely link your bank account using Teller"),

		shadcn.Card(shadcn.CardProps{},
			shadcn.CardContentFull(
				h.Div(
					h.ID("teller-connect"),
					h.Class("flex items-center justify-center py-12"),
				),

				// Teller Connect script
				h.Script(
					g.Attr("src", "https://cdn.teller.io/connect/connect.js"),
				),

				h.Script(
					g.Raw(`
						document.addEventListener('DOMContentLoaded', function() {
							var appId = '`+hdl.cfg.TellerAppID+`';
							var environment = '`+hdl.cfg.TellerEnvironment+`';
							
							if (!appId) {
								document.getElementById('teller-connect').innerHTML = '<p class="text-muted-foreground">Teller is not configured. Please add your Teller App ID to the environment.</p>';
								return;
							}
							
							var tellerConnect = TellerConnect.setup({
								applicationId: appId,
								environment: environment,
								onSuccess: function(enrollment) {
									// Show loading state immediately
									document.getElementById('teller-connect').innerHTML = '<div class="text-center"><div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary mb-4"></div><p class="text-foreground font-medium">Syncing your accounts...</p><p class="text-muted-foreground text-sm mt-2">This may take a moment</p></div>';
									// POST enrollment data so the access token never appears in the URL
									var f = document.createElement('form');
									f.method = 'POST'; f.action = '/teller/callback';
									[['enrollment_id', enrollment.enrollment.id], ['access_token', enrollment.accessToken]].forEach(function(p) {
										var i = document.createElement('input'); i.type = 'hidden'; i.name = p[0]; i.value = p[1]; f.appendChild(i);
									});
									document.body.appendChild(f); f.submit();
								},
								onExit: function() {
									window.location.href = '/accounts';
								},
								onError: function(error) {
									var msg = (error && error.message) || 'An error occurred connecting to your bank. Please try again.';
									var container = document.getElementById('teller-connect');
									container.innerHTML = '';
									var wrapper = document.createElement('div');
									wrapper.className = 'text-center py-8';
									var title = document.createElement('p');
									title.className = 'text-destructive font-medium mb-2';
									title.textContent = 'Connection failed';
									var detail = document.createElement('p');
									detail.className = 'text-muted-foreground text-sm';
									detail.textContent = msg;
									wrapper.appendChild(title);
									wrapper.appendChild(detail);
									container.appendChild(wrapper);
								}
							});

							var btn = document.createElement('button');
							btn.className = 'bg-indigo-600 text-white rounded-lg px-6 py-3 text-sm font-medium hover:bg-indigo-500 transition-colors';
							btn.innerText = 'Connect Bank Account';
							btn.onclick = function() {
								tellerConnect.open();
							};
							
							document.getElementById('teller-connect').appendChild(btn);
						});
					`),
				),
			),
		),

		// Info section
		h.Div(
			h.Class("mt-6"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.H3(h.Class("font-medium text-foreground mb-2"), g.Text("About Teller Connect")),
					h.P(h.Class("text-sm text-muted-foreground mb-4"),
						g.Text("Teller provides secure, read-only access to your bank accounts. Your credentials are never stored by this application."),
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

func (hdl *Handlers) TellerCallback(w http.ResponseWriter, r *http.Request) {
	enrollmentID := r.FormValue("enrollment_id")
	accessToken := r.FormValue("access_token")

	if enrollmentID == "" || accessToken == "" {
		http.Error(w, "Missing enrollment data", http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	syncService, err := hdl.newTellerSyncService()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use background context - sync can take a long time
	syncCtx := context.Background()

	// Sync accounts
	accounts, err := syncService.SyncAccounts(syncCtx, ledger.ID, accessToken)
	if err != nil {
		slog.ErrorContext(r.Context(), "Error syncing accounts", "err", err)

		// Check if this is an enrollment disconnection error
		if providers.IsConnectionDisconnectedError(err) {
			// Redirect to reconnect page
			redirectURL := "/teller/reconnect?enrollment_id=" + enrollmentID + "&message=Your bank connection needs to be re-authenticated"
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
			return
		}

		observability.CaptureFailure(r.Context(), err, observability.FailureOptions{
			Component: "teller_connect",
			Operation: "sync_accounts",
			Tags:      map[string]string{"enrollment_id": enrollmentID},
		})

		// For other errors, show a user-friendly error page
		// Check if it's a Teller API error with a specific message
		var errorMessage string
		if tellerErr, ok := err.(*sync.TellerAPIError); ok {
			errorMessage = tellerErr.Message
			// Provide more helpful messages for common errors
			if tellerErr.IsUnableToProcess() {
				errorMessage = "Teller is unable to process this account. This may be a temporary issue, or your account may need to be reconnected. Please try again in a few minutes, or reconnect your account."
			}
		} else {
			errorMessage = err.Error()
		}

		http.Error(w, "Failed to sync accounts: "+errorMessage, http.StatusInternalServerError)
		return
	}

	// Sync transactions for each account
	for _, acc := range accounts {
		if _, err := syncService.SyncTransactions(syncCtx, acc); err != nil {
			captureTellerError(r.Context(), err, "transaction sync failed after connect",
				observability.FailureOptions{Component: "teller_connect", Operation: "sync_transactions_after_connect", Tags: map[string]string{"account_id": acc.ID.String()}},
				"account_id", acc.ID,
			)
		}
	}

	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

func (hdl *Handlers) TellerSync(w http.ResponseWriter, r *http.Request) {
	enrollmentID := enrollmentIDFromRequest(r)

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find accounts with this enrollment
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	syncService, err := hdl.newTellerSyncService()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use background context - sync can take a long time
	syncCtx := context.Background()

	// First, re-sync account metadata (including last_four for transfer matching)
	var enrollmentDisconnected bool
	var firstAccountID string
	for _, acc := range accounts {
		if acc.TellerEnrollmentID == enrollmentID && acc.TellerToken() != "" {
			if firstAccountID == "" {
				firstAccountID = acc.ID.String()
			}
			_, err := syncService.SyncAccounts(syncCtx, ledger.ID, acc.TellerToken())
			if err != nil {
				if providers.IsConnectionDisconnectedError(err) {
					enrollmentDisconnected = true
					slog.WarnContext(r.Context(), "enrollment disconnected (MFA required), clearing credentials", "enrollment_id", enrollmentID)
				} else {
					captureTellerError(r.Context(), err, "account metadata sync failed",
						observability.FailureOptions{Component: "teller_sync", Operation: "sync_accounts", Tags: map[string]string{"enrollment_id": enrollmentID}},
						"enrollment_id", enrollmentID,
					)
				}
			}
			break // Only need to call once per enrollment (all accounts share the token)
		}
	}

	// If enrollment is disconnected, clear credentials and redirect to reconnect
	if enrollmentDisconnected {
		hdl.clearEnrollmentCredentials(r.Context(), enrollmentID)

		redirectURL := "/teller/reconnect?enrollment_id=" + enrollmentID
		if firstAccountID != "" {
			redirectURL += "&account_id=" + firstAccountID
		}

		htmxRedirect(w, r, redirectURL)
		return
	}

	var totalSynced int
	for _, acc := range accounts {
		if accountBelongsToEnrollment(acc, enrollmentID) {
			synced, err := syncService.SyncTransactions(syncCtx, acc)
			if err != nil {
				if providers.IsConnectionDisconnectedError(err) {
					// Handle disconnection detected during transaction sync
					hdl.clearEnrollmentCredentials(r.Context(), enrollmentID)

					redirectURL := "/teller/reconnect?enrollment_id=" + enrollmentID + "&account_id=" + acc.ID.String()
					htmxRedirect(w, r, redirectURL)
					return
				}
				captureTellerError(r.Context(), err, "transaction sync failed",
					observability.FailureOptions{Component: "teller_sync", Operation: "sync_transactions", Tags: map[string]string{"account_id": acc.ID.String(), "enrollment_id": enrollmentID}},
					"account_id", acc.ID,
				)
			}
			totalSynced += synced
		}
	}

	// Fetch institution logos for accounts missing them (independent of Teller)
	hdl.syncInstitutionLogos(syncCtx, accounts, enrollmentID)

	// Redirect back to the referring page, or default to /settings/banks
	redirectURL := "/settings/banks"
	if referer := r.Header.Get("Referer"); referer != "" {
		// Extract path from referer URL
		if strings.Contains(referer, "/accounts/") {
			redirectURL = "/accounts"
		} else if strings.Contains(referer, "/settings") {
			redirectURL = "/settings/banks"
		}
	}

	// HTMX response or redirect
	htmxRedirect(w, r, redirectURL)
}

// TellerSyncMulti syncs multiple enrollments (for institutions with multiple enrollments)
func (hdl *Handlers) TellerSyncMulti(w http.ResponseWriter, r *http.Request) {
	enrollmentIDsStr := r.FormValue("enrollment_ids")
	if enrollmentIDsStr == "" {
		slog.WarnContext(r.Context(), "No enrollment_ids provided in form")
		http.Error(w, "No enrollment IDs provided", http.StatusBadRequest)
		return
	}

	slog.InfoContext(r.Context(), "Received enrollment_ids", "enrollment_id", enrollmentIDsStr)
	enrollmentIDs := strings.Split(enrollmentIDsStr, ",")

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get all accounts BEFORE we start clearing credentials
	// This way we can find account IDs even after credentials are cleared
	allAccounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build a map of enrollment ID to account info for quick lookup
	enrollmentToAccount := make(map[string]struct {
		ID   string
		Name string
	})
	for _, acc := range allAccounts {
		enrollmentID := acc.ConnectionID
		if enrollmentID == "" {
			enrollmentID = acc.TellerEnrollmentID
		}
		if enrollmentID != "" {
			enrollmentToAccount[enrollmentID] = struct {
				ID   string
				Name string
			}{
				ID:   acc.ID.String(),
				Name: acc.Name,
			}
		}
	}

	var syncedCount int
	var missingTokenEnrollments []string
	var otherErrors int

	// Sync each enrollment, passing pre-fetched accounts to avoid redundant DB queries.
	for _, enrollmentID := range enrollmentIDs {
		enrollmentID = strings.TrimSpace(enrollmentID)
		if enrollmentID == "" {
			continue
		}
		slog.DebugContext(r.Context(), "Syncing enrollment", "enrollment_id", enrollmentID)
		if err := hdl.syncSingleEnrollment(r.Context(), ledger.ID, enrollmentID, allAccounts); err != nil {
			slog.ErrorContext(r.Context(), "Error syncing enrollment", "enrollment_id", enrollmentID, "err", err)
			// Classify the error: missing token and disconnected/unavailable errors both
			// require reconnection; everything else is a transient failure.
			if errors.Is(err, errNoAccessToken) || providers.IsConnectionDisconnectedError(err) {
				missingTokenEnrollments = append(missingTokenEnrollments, enrollmentID)
				hdl.clearEnrollmentCredentials(r.Context(), enrollmentID)
			} else {
				otherErrors++
			}
		} else {
			syncedCount++
			slog.DebugContext(r.Context(), "Successfully synced enrollment", "enrollment_id", enrollmentID)
		}
	}

	slog.InfoContext(r.Context(), "teller-sync-multi completed", "successful", syncedCount, "missing_tokens", len(missingTokenEnrollments), "other_errors", otherErrors)

	// If any enrollments are missing tokens, redirect to reconnect the first one
	if len(missingTokenEnrollments) > 0 {
		firstEnrollmentID := missingTokenEnrollments[0]

		// Use the map we built earlier to find account info
		var accountID string
		var accountName string
		if accountInfo, found := enrollmentToAccount[firstEnrollmentID]; found {
			accountID = accountInfo.ID
			accountName = accountInfo.Name
		}

		// Build redirect URL - if we found an account, link to it; otherwise general connect
		redirectURL := "/teller/reconnect?enrollment_id=" + firstEnrollmentID
		if accountID != "" {
			redirectURL += "&account_id=" + accountID
		}
		// Add a message parameter to show user why they're being redirected
		var message string
		if len(missingTokenEnrollments) > 1 {
			message = "Some accounts need to be reconnected to sync"
		} else if accountName != "" {
			message = "Your " + accountName + " account needs to be reconnected to sync"
		} else {
			message = "Your account needs to be reconnected to sync"
		}
		redirectURL += "&message=" + url.QueryEscape(message)

		htmxRedirect(w, r, redirectURL)
		return
	}

	// If all succeeded or only non-token errors, redirect back
	redirectURL := "/settings/banks"
	htmxRedirect(w, r, redirectURL)
}

// syncSingleEnrollment syncs a single enrollment. allAccounts is the pre-fetched list of
// accounts for the ledger (obtained by the caller to avoid redundant DB queries in a loop).
func (hdl *Handlers) syncSingleEnrollment(ctx context.Context, ledgerID uuid.UUID, enrollmentID string, allAccounts []*models.Account) error {
	// Find accounts matching this enrollment
	var matchingAccounts []*models.Account
	for _, acc := range allAccounts {
		if accountBelongsToEnrollment(acc, enrollmentID) {
			matchingAccounts = append(matchingAccounts, acc)
		}
	}

	if len(matchingAccounts) == 0 {
		slog.InfoContext(ctx, "No accounts found for enrollment", "enrollment_id", enrollmentID)
		return fmt.Errorf("no accounts found for enrollment %s", enrollmentID)
	}

	slog.DebugContext(ctx, "Found accounts for enrollment", "count", len(matchingAccounts), "enrollment_id", enrollmentID)

	syncService, err := hdl.newTellerSyncService()
	if err != nil {
		return fmt.Errorf("creating Teller client: %w", err)
	}

	syncCtx := context.Background()

	// Find an account with a Teller access token to use for metadata sync.
	var accessToken string
	for _, acc := range matchingAccounts {
		if t := acc.TellerToken(); t != "" {
			accessToken = t
			break
		}
	}

	if accessToken == "" {
		slog.WarnContext(ctx, "no access token found for enrollment; accounts may need to be reconnected", "enrollment_id", enrollmentID)
		return fmt.Errorf("enrollment %s: %w", enrollmentID, errNoAccessToken)
	}

	slog.DebugContext(ctx, "syncing account metadata for enrollment", "enrollment_id", enrollmentID)
	syncedAccounts, err := syncService.SyncAccounts(syncCtx, ledgerID, accessToken)
	if err != nil {
		if providers.IsConnectionDisconnectedError(err) {
			return fmt.Errorf("enrollment disconnected - needs reconnection: %w", err)
		}
		return fmt.Errorf("syncing accounts: %w", err)
	}
	slog.DebugContext(ctx, "synced accounts metadata", "count", len(syncedAccounts))

	// Reload accounts from database after metadata sync to pick up updated tokens and details.
	// logoAccounts tracks the full account list for logo fetching (falls back to allAccounts on error).
	logoAccounts := allAccounts
	if reloaded, err := hdl.accounts.GetByLedgerID(ctx, ledgerID); err == nil {
		logoAccounts = reloaded
		matchingAccounts = matchingAccounts[:0]
		for _, acc := range reloaded {
			if accountBelongsToEnrollment(acc, enrollmentID) {
				matchingAccounts = append(matchingAccounts, acc)
			}
		}
		slog.DebugContext(ctx, "reloaded accounts for enrollment after metadata sync", "count", len(matchingAccounts))
	} else {
		slog.WarnContext(ctx, "failed to reload accounts after metadata sync, using pre-sync data", "enrollment_id", enrollmentID, "err", err)
	}

	// Sync transactions for all accounts in this enrollment
	var totalTxns int
	for _, acc := range matchingAccounts {
		// Ensure account has access token (should be set after SyncAccounts)
		if acc.AccessToken == "" && acc.TellerAccessToken == "" {
			slog.WarnContext(ctx, "account still has no access token after metadata sync", "name", acc.Name)
			acc.AccessToken = accessToken
			acc.TellerAccessToken = accessToken
		}
		slog.DebugContext(ctx, "syncing transactions for account", "account", acc.Name, "id", acc.ID)
		count, err := syncService.SyncTransactions(syncCtx, acc)
		if err != nil {
			captureTellerError(ctx, err, "transaction sync failed",
				observability.FailureOptions{Component: "teller_sync_multi", Operation: "sync_transactions", Tags: map[string]string{"account_id": acc.ID.String(), "enrollment_id": enrollmentID}},
				"account", acc.Name,
			)
			continue
		}
		totalTxns += count
		slog.DebugContext(ctx, "synced transactions for account", "count", count, "account", acc.Name)
	}

	slog.InfoContext(ctx, "total transactions synced for enrollment", "enrollment_id", enrollmentID, "total", totalTxns)

	// Fetch institution logos using latest account data
	hdl.syncInstitutionLogos(syncCtx, logoAccounts, enrollmentID)

	return nil
}

// TellerFullResyncMulti does a full resync of multiple enrollments
func (hdl *Handlers) TellerFullResyncMulti(w http.ResponseWriter, r *http.Request) {
	enrollmentIDsStr := r.FormValue("enrollment_ids")
	if enrollmentIDsStr == "" {
		http.Error(w, "No enrollment IDs provided", http.StatusBadRequest)
		return
	}

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	enrollmentIDs := strings.Split(enrollmentIDsStr, ",")

	for _, enrollmentID := range enrollmentIDs {
		enrollmentID = strings.TrimSpace(enrollmentID)
		if enrollmentID == "" {
			continue
		}
		hdl.resyncSingleEnrollment(r.Context(), ledger.ID, enrollmentID)
	}

	redirectURL := "/settings/banks"
	htmxRedirect(w, r, redirectURL)
}

// resyncSingleEnrollment does a full resync of a single enrollment
func (hdl *Handlers) resyncSingleEnrollment(ctx context.Context, ledgerID uuid.UUID, enrollmentID string) {
	accounts, err := hdl.accounts.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		slog.ErrorContext(ctx, "Error getting accounts", "err", err)
		return
	}

	syncService, err := hdl.newTellerSyncService()
	if err != nil {
		slog.ErrorContext(ctx, "Error creating Teller client", "err", err)
		return
	}

	syncCtx := context.Background()

	for _, acc := range accounts {
		if accountBelongsToEnrollment(acc, enrollmentID) {
			if _, err := syncService.FullResync(syncCtx, acc); err != nil {
				captureTellerError(ctx, err, "full resync failed",
					observability.FailureOptions{Component: "teller_full_resync", Operation: "full_resync", Tags: map[string]string{"account_id": acc.ID.String(), "enrollment_id": enrollmentID}},
					"account_id", acc.ID,
				)
			}
		}
	}
}

// TellerDisconnectMulti disconnects multiple enrollments
func (hdl *Handlers) TellerDisconnectMulti(w http.ResponseWriter, r *http.Request) {
	enrollmentIDsStr := r.FormValue("enrollment_ids")
	if enrollmentIDsStr == "" {
		http.Error(w, "No enrollment IDs provided", http.StatusBadRequest)
		return
	}

	enrollmentIDs := strings.Split(enrollmentIDsStr, ",")

	for _, enrollmentID := range enrollmentIDs {
		enrollmentID = strings.TrimSpace(enrollmentID)
		if enrollmentID == "" {
			continue
		}
		hdl.clearEnrollmentCredentials(r.Context(), enrollmentID)
	}

	redirectURL := "/settings/banks"
	htmxRedirect(w, r, redirectURL)
}

// clearEnrollmentCredentials clears Teller credentials for all accounts with the given enrollment.
// IMPORTANT: We do NOT clear TellerAccountID/ExternalAccountID because Teller account IDs are persistent.
// This allows us to match accounts by ID when reconnecting, even after credentials are cleared.
func (hdl *Handlers) clearEnrollmentCredentials(ctx context.Context, enrollmentID string) {
	accounts, err := hdl.accounts.GetByConnectionID(ctx, enrollmentID)
	if err != nil {
		slog.ErrorContext(ctx, "Error finding accounts for enrollment", "enrollment_id", enrollmentID, "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "teller_enrollment",
			Operation: "clear_credentials_lookup",
		})
		return
	}
	hdl.clearCredentialsForAccounts(ctx, accounts)
}

// clearCredentialsForAccounts wipes transient Teller token fields for each account and
// persists the change. Continues on per-account errors so a single DB failure does not
// silently leave other accounts with stale tokens.
// DO NOT clear TellerAccountID or ExternalAccountID — they are persistent and needed for reconnect matching.
func (hdl *Handlers) clearCredentialsForAccounts(ctx context.Context, accounts []*models.Account) {
	for _, acc := range accounts {
		acc.ConnectionID = ""
		acc.AccessToken = ""
		acc.TellerEnrollmentID = ""
		acc.TellerAccessToken = ""

		if err := hdl.accounts.Update(ctx, acc); err != nil {
			slog.ErrorContext(ctx, "failed to clear Teller credentials for account", "id", acc.ID, "err", err)
			observability.CaptureFailure(ctx, err, observability.FailureOptions{
				Component: "teller_enrollment",
				Operation: "clear_credentials_update",
				Tags:      map[string]string{"account_id": acc.ID.String()},
			})
		} else {
			slog.DebugContext(ctx, "cleared Teller credentials for account", "id", acc.ID, "name", acc.Name, "teller_account_id", acc.TellerAccountID)
		}
	}
}

// syncInstitutionLogos fetches logos for institutions that don't have them
func (hdl *Handlers) syncInstitutionLogos(ctx context.Context, accounts []*models.Account, enrollmentID string) {
	if hdl.cfg == nil || hdl.cfg.FirecrawlAPIKey == "" {
		return
	}

	// Filter to accounts in this enrollment that need logos
	var needsLogo []*models.Account
	for _, acc := range accounts {
		if accountBelongsToEnrollment(acc, enrollmentID) {
			// Skip only if a stored logo already exists.
			// Institution IDs alone are not reliable for Teller-hosted logo URLs.
			if acc.InstitutionLogoURL != "" {
				continue
			}
			needsLogo = append(needsLogo, acc)
		}
	}

	if len(needsLogo) == 0 {
		return
	}

	// Group by institution name
	byInstitution := make(map[string][]*models.Account)
	for _, acc := range needsLogo {
		if acc.InstitutionName != "" {
			byInstitution[acc.InstitutionName] = append(byInstitution[acc.InstitutionName], acc)
		}
	}

	if len(byInstitution) == 0 {
		return
	}

	// Use the handler's cached Firecrawl client
	firecrawl := hdl.firecrawl

	// Use the handler's logo client if available, otherwise create a temporary one
	var logoStore *enrichment.LogoStore
	if hdl.logoClient != nil {
		logoStore = hdl.logoClient.GetLogoStore()
	} else {
		// Fallback: create a temporary logo client (will use local storage)
		logoClient, err := enrichment.NewLogoClient(hdl.cfg)
		if err != nil {
			slog.WarnContext(ctx, "Could not initialize logo client", "err", err)
			return
		}
		logoStore = logoClient.GetLogoStore()
	}

	for institutionName, accs := range byInstitution {
		slog.DebugContext(ctx, "fetching logo for institution", "institution", institutionName)

		// Search for the institution website
		info, err := firecrawl.SearchAndExtract(ctx, institutionName+" bank", "", "")
		if err != nil {
			slog.WarnContext(ctx, "could not fetch logo", "institution", institutionName, "err", err)
			continue
		}

		if info == nil || info.LogoURL == "" {
			slog.DebugContext(ctx, "no logo found", "institution", institutionName)
			continue
		}

		// Download and store the logo locally
		localPath, err := logoStore.DownloadAndStore(ctx, info.LogoURL)
		if err != nil {
			slog.WarnContext(ctx, "could not download logo", "institution", institutionName, "err", err)
			continue
		}

		slog.DebugContext(ctx, "downloaded logo", "institution", institutionName, "path", localPath)

		// Update all accounts for this institution with the logo
		for _, acc := range accs {
			acc.InstitutionLogoURL = localPath
			if err := hdl.accounts.Update(ctx, acc); err != nil {
				slog.WarnContext(ctx, "Could not update logo for account", "id", acc.ID, "err", err)
			}
		}
	}
}

// TellerFullResync performs a complete resync, updating existing transactions with new Teller data
// This is useful for backfilling new fields like category, running_balance, etc.
func (hdl *Handlers) TellerFullResync(w http.ResponseWriter, r *http.Request) {
	enrollmentID := chi.URLParam(r, "enrollmentId")

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find accounts with this enrollment
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	syncService, err := hdl.newTellerSyncService()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use a background context for sync operations - they can take a long time
	// and shouldn't be canceled when the HTTP request times out
	syncCtx := context.Background()

	// First, re-sync account metadata
	var enrollmentDisconnected bool
	var firstAccountID string
	for _, acc := range accounts {
		if acc.TellerEnrollmentID == enrollmentID && acc.TellerToken() != "" {
			if firstAccountID == "" {
				firstAccountID = acc.ID.String()
			}
			_, err := syncService.SyncAccounts(syncCtx, ledger.ID, acc.TellerToken())
			if err != nil {
				if providers.IsConnectionDisconnectedError(err) {
					enrollmentDisconnected = true
					slog.WarnContext(r.Context(), "enrollment disconnected (MFA required), clearing credentials", "enrollment_id", enrollmentID)
				} else if sync.IsTellerTransientError(err) {
					slog.WarnContext(r.Context(), "transient teller error syncing account metadata, will retry", "enrollment_id", enrollmentID)
				} else {
					slog.WarnContext(r.Context(), "account metadata sync failed during full resync", "enrollment_id", enrollmentID, "err", err)
					observability.CaptureFailure(r.Context(), err, observability.FailureOptions{
						Component: "teller_full_resync",
						Operation: "sync_accounts",
						Tags:      map[string]string{"enrollment_id": enrollmentID},
					})
				}
			}
			break
		}
	}

	// If enrollment is disconnected, clear credentials and redirect to reconnect
	if enrollmentDisconnected {
		hdl.clearEnrollmentCredentials(r.Context(), enrollmentID)

		redirectURL := "/teller/reconnect?enrollment_id=" + enrollmentID
		if firstAccountID != "" {
			redirectURL += "&account_id=" + firstAccountID
		}

		htmxRedirect(w, r, redirectURL)
		return
	}

	var totalSynced int
	for _, acc := range accounts {
		if accountBelongsToEnrollment(acc, enrollmentID) {
			// Delete all transactions and re-import fresh from Teller
			synced, err := syncService.DeleteAndResync(syncCtx, acc)
			if err != nil {
				if providers.IsConnectionDisconnectedError(err) {
					hdl.clearEnrollmentCredentials(r.Context(), enrollmentID)
					redirectURL := "/teller/reconnect?enrollment_id=" + enrollmentID + "&account_id=" + acc.ID.String()
					htmxRedirect(w, r, redirectURL)
					return
				}
				if sync.IsTellerTransientError(err) {
					slog.WarnContext(r.Context(), "transient teller error during full resync, will retry", "account_id", acc.ID)
				} else {
					slog.WarnContext(r.Context(), "full resync failed for account", "account_id", acc.ID, "err", err)
					observability.CaptureFailure(r.Context(), err, observability.FailureOptions{
						Component: "teller_full_resync",
						Operation: "delete_and_resync",
						Tags:      map[string]string{"account_id": acc.ID.String(), "enrollment_id": enrollmentID},
					})
				}
			}
			totalSynced += synced
		}
	}

	// HTMX response or redirect
	htmxRedirect(w, r, "/settings/banks")
}

// TellerReconnect shows a page explaining that the bank connection needs re-authentication
// This is shown when syncing fails due to MFA requirements or expired credentials
func (hdl *Handlers) TellerReconnect(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	accountID := r.URL.Query().Get("account_id")
	message := r.URL.Query().Get("message")

	// Build the reconnect URL - either link a specific account or general connect
	var reconnectURL string
	var accountName string
	if accountID != "" {
		reconnectURL = "/teller/link/" + accountID
		// Try to get the account name for a better message
		if accUUID, err := uuid.Parse(accountID); err == nil {
			if acc, err := hdl.accounts.GetByID(r.Context(), accUUID); err == nil {
				accountName = acc.Name
			}
		}
	} else {
		reconnectURL = "/teller/connect"
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
						// Warning icon
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
						h.H3(h.Class("font-medium text-white mb-2"), g.Text("How to fix it")),
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
							h.Class("text-sm text-muted-foreground hover:text-foreground transition-colors"),
							g.Text("Back to Accounts"),
						),
					),
				),
			),
		),

		// Info card
		h.Div(
			h.Class("mt-6"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.H3(h.Class("font-medium text-foreground mb-2"), g.Text("About Bank Security")),
					h.P(h.Class("text-sm text-muted-foreground"),
						g.Text("Banks use multi-factor authentication (MFA) to protect your accounts. When you reconnect, your credentials are handled securely by Teller and are never stored by this application."),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

func (hdl *Handlers) TellerDisconnect(w http.ResponseWriter, r *http.Request) {
	enrollmentID := enrollmentIDFromRequest(r)

	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find and deactivate accounts with this enrollment
	accounts, err := hdl.accounts.GetByLedgerID(r.Context(), ledger.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var toDisconnect []*models.Account
	for _, acc := range accounts {
		if accountBelongsToEnrollment(acc, enrollmentID) {
			toDisconnect = append(toDisconnect, acc)
		}
	}
	hdl.clearCredentialsForAccounts(r.Context(), toDisconnect)

	// Redirect back to settings/banks after disconnect
	http.Redirect(w, r, "/settings/banks", http.StatusSeeOther)
}

// TellerLink renders the Teller Connect page for linking to an existing account
func (hdl *Handlers) TellerLink(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	accountID := chi.URLParam(r, "accountId")

	// Verify the account exists and belongs to this user's ledger
	_, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render page with Teller Connect embedded, passing the accountId to the callback
	page := layouts.AppLayout("Link Bank Account", user.Email, user.ID.String(),
		shadcn.PageHeader("Link Bank Account", "Connect your existing account to a bank"),

		shadcn.Card(shadcn.CardProps{},
			shadcn.CardContentFull(
				h.Div(
					h.ID("teller-connect"),
					h.Class("flex items-center justify-center py-12"),
				),

				// Teller Connect script
				h.Script(
					g.Attr("src", "https://cdn.teller.io/connect/connect.js"),
				),

				h.Script(
					g.Raw(`
						document.addEventListener('DOMContentLoaded', function() {
							var appId = '`+hdl.cfg.TellerAppID+`';
							var environment = '`+hdl.cfg.TellerEnvironment+`';
							var accountId = '`+accountID+`';
							
							if (!appId) {
								document.getElementById('teller-connect').innerHTML = '<p class="text-muted-foreground">Teller is not configured. Please add your Teller App ID to the environment.</p>';
								return;
							}
							
							var tellerConnect = TellerConnect.setup({
								applicationId: appId,
								environment: environment,
								onSuccess: function(enrollment) {
									try {
										// Show loading state immediately
										document.getElementById('teller-connect').innerHTML = '<div class="text-center"><div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary mb-4"></div><p class="text-foreground font-medium">Linking your account...</p><p class="text-muted-foreground text-sm mt-2">This may take a moment</p></div>';
										
										// Get enrollment ID - handle different response structures
										var enrollmentId = enrollment.enrollment ? enrollment.enrollment.id : enrollment.id;
										
										// Get teller account ID if available (some institutions include it, some don't)
										var tellerAccountId = '';
										if (enrollment.enrollment && enrollment.enrollment.accounts && enrollment.enrollment.accounts.length > 0) {
											tellerAccountId = enrollment.enrollment.accounts[0].id;
										} else if (enrollment.accounts && enrollment.accounts.length > 0) {
											tellerAccountId = enrollment.accounts[0].id;
										}
										
										// POST enrollment data so the access token never appears in the URL
										var f = document.createElement('form');
										f.method = 'POST'; f.action = '/teller/link-callback';
										var fields = [['enrollment_id', enrollmentId], ['access_token', enrollment.accessToken], ['account_id', accountId]];
										if (tellerAccountId) { fields.push(['teller_account_id', tellerAccountId]); }
										fields.forEach(function(p) {
											var i = document.createElement('input'); i.type = 'hidden'; i.name = p[0]; i.value = p[1]; f.appendChild(i);
										});
										document.body.appendChild(f); f.submit();
									} catch (e) {
										console.error('Error in Teller onSuccess:', e);
										console.error('Enrollment object:', enrollment);
										alert('Error processing Teller response: ' + e.message + '. Check console for details.');
									}
								},
								onExit: function() {
									window.location.href = '/accounts/' + accountId;
								},
								onError: function(error) {
									var msg = (error && error.message) || 'An error occurred linking your bank account. Please try again.';
									var container = document.getElementById('teller-connect');
									container.innerHTML = '';
									var wrapper = document.createElement('div');
									wrapper.className = 'text-center py-8';
									var title = document.createElement('p');
									title.className = 'text-destructive font-medium mb-2';
									title.textContent = 'Connection failed';
									var detail = document.createElement('p');
									detail.className = 'text-muted-foreground text-sm';
									detail.textContent = msg;
									wrapper.appendChild(title);
									wrapper.appendChild(detail);
									container.appendChild(wrapper);
								}
							});

							var btn = document.createElement('button');
							btn.className = 'bg-indigo-600 text-white rounded-lg px-6 py-3 text-sm font-medium hover:bg-indigo-500 transition-colors';
							btn.innerText = 'Select Bank Account to Link';
							btn.onclick = function() {
								tellerConnect.open();
							};
							
							document.getElementById('teller-connect').appendChild(btn);
						});
					`),
				),
			),
		),

		// Info section
		h.Div(
			h.Class("mt-6"),
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardContentFull(
					h.H3(h.Class("font-medium text-white mb-2"), g.Text("Link Your Account")),
					h.P(h.Class("text-sm text-muted-foreground mb-4"),
						g.Text("Select a bank account to link. Transactions will be synced and deduplicated automatically."),
					),
					h.Ul(
						h.Class("text-sm text-muted-foreground space-y-2"),
						h.Li(g.Text("• Existing transactions are preserved")),
						h.Li(g.Text("• Duplicate transactions are automatically detected")),
						h.Li(g.Text("• You can disconnect anytime")),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

// TellerLinkCallback handles the callback after linking Teller to an existing account
func (hdl *Handlers) TellerLinkCallback(w http.ResponseWriter, r *http.Request) {
	enrollmentID := r.FormValue("enrollment_id")
	accessToken := r.FormValue("access_token")
	accountID := r.FormValue("account_id")
	tellerAccountID := r.FormValue("teller_account_id") // May be empty for some institutions

	if enrollmentID == "" || accessToken == "" || accountID == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Get the existing account
	accountUUID, err := uuid.Parse(accountID)
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountUUID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Use the pre-initialized Teller client (avoids per-request TLS certificate parsing)
	tellerClient := hdl.tellerClient
	if tellerClient == nil {
		http.Error(w, "Teller not configured", http.StatusServiceUnavailable)
		return
	}

	// Fetch accounts from Teller — used to resolve tellerAccountID when absent, and for institution details.
	var tellerAccounts []sync.TellerAccount

	if tellerAccountID == "" {
		slog.DebugContext(r.Context(), "No teller_account_id provided, fetching accounts from Teller...")
		tellerAccounts, err = tellerClient.GetAccounts(r.Context(), accessToken)
		if err != nil {
			http.Error(w, "Failed to fetch accounts from Teller: "+err.Error(), http.StatusInternalServerError)
			return
		}

		slog.DebugContext(r.Context(), "Fetched accounts from Teller", "count", len(tellerAccounts))

		if len(tellerAccounts) == 0 {
			http.Error(w, "No accounts found in Teller enrollment", http.StatusBadRequest)
			return
		}

		// Try to match by name first
		for _, ta := range tellerAccounts {
			slog.DebugContext(r.Context(), "[teller-link] Teller account", "name", ta.Name, "id", ta.ID, "institution", ta.Institution.Name)
			if ta.Name == account.Name {
				tellerAccountID = ta.ID
				slog.DebugContext(r.Context(), "[teller-link] Matched by name", "account", account.Name, "id", ta.ID)
				break
			}
		}

		// If no name match, use the first account (common for single-account connections)
		if tellerAccountID == "" {
			tellerAccountID = tellerAccounts[0].ID
			slog.DebugContext(r.Context(), "[teller-link] Using first account", "name", tellerAccounts[0].Name, "id", tellerAccountID)
		}
	}

	// Update the account with Teller credentials
	account.ExternalAccountID = tellerAccountID
	account.ConnectionID = enrollmentID
	account.AccessToken = accessToken
	account.Provider = "teller"
	// Also set old fields for backward compatibility
	account.TellerAccountID = tellerAccountID
	account.TellerEnrollmentID = enrollmentID
	account.TellerAccessToken = accessToken

	// Get institution name and other details from Teller (reuse accounts fetched above when available).
	if tellerAccounts == nil {
		var getErr error
		tellerAccounts, getErr = tellerClient.GetAccounts(r.Context(), accessToken)
		if getErr != nil {
			slog.WarnContext(r.Context(), "teller: failed to fetch accounts for enrichment", "err", getErr)
		}
	}
	for _, ta := range tellerAccounts {
		if ta.ID == tellerAccountID {
			if ta.Institution.Name != "" {
				account.InstitutionName = ta.Institution.Name
			}
			if ta.LastFour != "" {
				account.LastFour = ta.LastFour
			}
			break
		}
	}

	if err := hdl.accounts.Update(r.Context(), account); err != nil {
		http.Error(w, "Failed to update account: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "[teller-link] Successfully linked account", "name", account.Name, "id", account.ID, "teller_id", tellerAccountID)

	// Sync transactions for this account (with deduplication)
	syncService, syncErr := hdl.newTellerSyncService()
	if syncErr != nil {
		slog.WarnContext(r.Context(), "teller sync service unavailable after link", "err", syncErr)
	} else {
		synced, err := syncService.SyncTransactions(r.Context(), account)
		if err != nil {
			captureTellerError(r.Context(), err, "failed to sync transactions after link",
				observability.FailureOptions{Component: "teller_link", Operation: "sync_transactions_after_link", Tags: map[string]string{"account_id": account.ID.String()}},
				"account_id", account.ID,
			)
		} else {
			slog.DebugContext(r.Context(), "synced transactions for account", "count", synced, "account", account.Name)
		}
	}

	http.Redirect(w, r, "/accounts/"+accountID, http.StatusSeeOther)
}

// TellerWebhook payload types

type TellerWebhookPayload struct {
	ID        string          `json:"id"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
}

type TellerEnrollmentDisconnectedPayload struct {
	EnrollmentID string `json:"enrollment_id"`
	Reason       string `json:"reason"`
}

type TellerTransactionsPayload struct {
	AccountID string `json:"account_id"`
}

// TellerWebhook handles incoming webhooks from Teller
// POST /api/teller/webhook
func (hdl *Handlers) TellerWebhook(w http.ResponseWriter, r *http.Request) {
	// Read the raw body for signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to read body", "err", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify signature if webhook secret is configured
	if hdl.cfg.TellerWebhookSecret != "" {
		signature := r.Header.Get("Teller-Signature")
		if !hdl.verifyTellerSignature(r.Context(), signature, body) {
			slog.WarnContext(r.Context(), "invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse the webhook payload
	webhook, ok := unmarshalWebhookPayload[TellerWebhookPayload](r.Context(), body, "webhook")
	if !ok {
		http.Error(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	slog.InfoContext(r.Context(), "received webhook event", "type", webhook.Type, "id", webhook.ID)

	// Acknowledge receipt immediately — Teller expects a fast 200 and will retry on timeout.
	// enrollment.disconnected is a quick DB write and stays synchronous.
	// transactions.processed triggers a full Teller API + DB sync, so it runs in the background.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))

	switch webhook.Type {
	case "webhook.test":
		slog.InfoContext(r.Context(), "Test webhook received")

	case "enrollment.disconnected":
		hdl.handleEnrollmentDisconnected(r.Context(), webhook.Payload)

	case "transactions.processed":
		payload := webhook.Payload
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			defer observability.RecoverAndLog(ctx, "teller_webhook_transactions_processed")
			hdl.handleTransactionsProcessed(ctx, payload)
		}()

	default:
		slog.InfoContext(r.Context(), "unhandled event type", "type", webhook.Type)
	}
}

// verifyTellerSignature verifies the webhook signature from Teller
// The Teller-Signature header format: t=<timestamp>,v1=<signature>[,v1=<signature>...]
func (hdl *Handlers) verifyTellerSignature(ctx context.Context, signatureHeader string, body []byte) bool {
	if signatureHeader == "" {
		return false
	}

	// Parse the signature header
	var timestamp string
	var signatures []string

	parts := strings.Split(signatureHeader, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			timestamp = strings.TrimPrefix(part, "t=")
		} else if strings.HasPrefix(part, "v1=") {
			signatures = append(signatures, strings.TrimPrefix(part, "v1="))
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		slog.WarnContext(ctx, "Missing timestamp or signature in header")
		return false
	}

	// Check timestamp is not too old (prevent replay attacks - 5 minute window)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		slog.WarnContext(ctx, "Invalid timestamp", "err", err)
		return false
	}

	age := time.Now().Unix() - ts
	if age > 300 || age < -60 { // 5 minutes old or 1 minute in future
		slog.WarnContext(ctx, "timestamp out of range", "age_seconds", age)
		return false
	}

	// Compute expected signature: HMAC-SHA256(secret, timestamp + "." + body)
	signedMessage := timestamp + "." + string(body)
	mac := hmac.New(sha256.New, []byte(hdl.cfg.TellerWebhookSecret))
	mac.Write([]byte(signedMessage))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Check if any of the provided signatures match
	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expectedSig)) {
			return true
		}
	}

	slog.WarnContext(ctx, "Signature mismatch")
	return false
}

// unmarshalWebhookPayload decodes JSON into a typed value, logging on failure.
func unmarshalWebhookPayload[T any](ctx context.Context, data []byte, eventType string) (T, bool) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		slog.ErrorContext(ctx, "failed to parse webhook payload", "type", eventType, "err", err)
		return v, false
	}
	return v, true
}

// handleEnrollmentDisconnected handles the enrollment.disconnected event
func (hdl *Handlers) handleEnrollmentDisconnected(ctx context.Context, payload json.RawMessage) {
	data, ok := unmarshalWebhookPayload[TellerEnrollmentDisconnectedPayload](ctx, payload, "enrollment.disconnected")
	if !ok {
		return
	}

	slog.WarnContext(ctx, "enrollment disconnected", "enrollment_id", data.EnrollmentID, "reason", data.Reason)
	hdl.clearEnrollmentCredentials(ctx, data.EnrollmentID)
}

// handleTransactionsProcessed handles the transactions.processed event
func (hdl *Handlers) handleTransactionsProcessed(ctx context.Context, payload json.RawMessage) {
	data, ok := unmarshalWebhookPayload[TellerTransactionsPayload](ctx, payload, "transactions.processed")
	if !ok {
		return
	}

	slog.InfoContext(ctx, "Transactions processed for account", "teller_account_id", data.AccountID)

	// Find the account by Teller account ID.
	// pgx.ErrNoRows is expected: Teller sends webhooks for all enrollments including
	// accounts not yet imported or already deleted. Treat as a warning, not a failure.
	account, err := hdl.accounts.GetByExternalAccountID(ctx, data.AccountID)
	if errors.Is(err, pgx.ErrNoRows) {
		slog.WarnContext(ctx, "teller webhook: account not found, skipping", "teller_account_id", data.AccountID)
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "Failed to find account for teller_account_id", "teller_account_id", data.AccountID, "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "teller_webhook",
			Operation: "transactions_processed_lookup",
		})
		return
	}

	// Sync transactions for this account
	syncService, err := hdl.newTellerSyncService()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create Teller client", "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "teller_webhook",
			Operation: "teller_client_init",
		})
		return
	}
	synced, err := syncService.SyncTransactions(ctx, account)
	if err != nil {
		if providers.IsConnectionDisconnectedError(err) {
			slog.WarnContext(ctx, "teller webhook: connection disconnected, marking account", "account_id", account.ID, "enrollment_id", account.ConnectionID)
			if dbErr := hdl.accounts.SetConnectionStatus(ctx, account.ID, "disconnected"); dbErr != nil {
				slog.WarnContext(ctx, "failed to mark account disconnected", "account_id", account.ID, "err", dbErr)
			}
			return
		}
		if sync.IsTellerTransientError(err) {
			// Teller server error — transient outage, not an application bug.
			// Background sync worker will retry on the next cycle.
			slog.WarnContext(ctx, "teller webhook: transient API error, skipping capture", "account_id", account.ID, "err", err)
			return
		}
		slog.ErrorContext(ctx, "Failed to sync transactions", "err", err)
		observability.CaptureFailure(ctx, err, observability.FailureOptions{
			Component: "teller_webhook",
			Operation: "transactions_processed_sync",
			Tags: map[string]string{
				"account_id": account.ID.String(),
			},
		})
		return
	}

	slog.DebugContext(ctx, "synced transactions for account", "count", synced, "account_id", account.ID)
}
