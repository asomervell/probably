package mcp

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/oauth"
	"github.com/google/uuid"
)

// AuthHandler handles OAuth 2.1 with PKCE authentication for ChatGPT Apps
type AuthHandler struct {
	cfg         *config.Config
	db          *db.DB
	ab          *authboss.Authboss
	tokenStore  *oauth.TokenStore
	clientStore *OAuthClientStore
	userStore   *models.UserStore
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config, database *db.DB, ab *authboss.Authboss) (*AuthHandler, error) {
	apiKeyStore := models.NewAPIKeyStore(database.Pool)

	return &AuthHandler{
		cfg:         cfg,
		db:          database,
		ab:          ab,
		tokenStore:  oauth.NewTokenStore(database.Pool, apiKeyStore),
		clientStore: NewOAuthClientStore(database.Pool),
		userStore:   models.NewUserStore(database.Pool),
	}, nil
}

// HandleAuthorization handles OAuth 2.1 authorization request with PKCE
// GET /mcp/auth?response_type=code&client_id=...&redirect_uri=...&scope=...&state=...&code_challenge=...&code_challenge_method=S256
func (h *AuthHandler) HandleAuthorization(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	responseType := r.URL.Query().Get("response_type")
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	scope := r.URL.Query().Get("scope")
	state := r.URL.Query().Get("state")
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")

	// Validate required parameters
	if responseType != "code" {
		http.Error(w, "invalid_response_type", http.StatusBadRequest)
		return
	}

	if clientID == "" {
		http.Error(w, "invalid_client", http.StatusBadRequest)
		return
	}

	if redirectURI == "" {
		http.Error(w, "invalid_redirect_uri", http.StatusBadRequest)
		return
	}

	// Validate redirect URI is registered for this client
	if !h.clientStore.ValidateRedirectURI(r.Context(), clientID, redirectURI) {
		http.Error(w, "invalid_redirect_uri", http.StatusBadRequest)
		return
	}

	// Validate PKCE parameters
	if codeChallenge == "" || codeChallengeMethod != "S256" {
		http.Error(w, "invalid_request: PKCE code_challenge required with method S256", http.StatusBadRequest)
		return
	}

	// Parse scopes
	scopes := []string{}
	if scope != "" {
		scopes = strings.Fields(scope)
	}

	// Default scopes if none provided
	if len(scopes) == 0 {
		scopes = []string{"read:transactions", "read:accounts", "read:financial"}
	}

	// Check if user is already authenticated (via Authboss session)
	// For ChatGPT Apps, we need to redirect to Probably login if not authenticated
	user := h.getUserFromSession(r)
	if user == nil {
		// User is not authenticated - redirect to Probably login
		// Store the OAuth request parameters in a way that survives the login redirect
		// We'll use the state parameter to reconstruct the OAuth request after login
		loginURL := fmt.Sprintf("%s/auth/login?redirect=%s", h.cfg.BaseURL, url.QueryEscape(r.URL.String()))
		http.Redirect(w, r, loginURL, http.StatusFound)
		return
	}

	// User is authenticated - allow OAuth flow to complete without subscription check
	// Subscription requirement will be enforced in the MCP tools/chat interface instead
	// This allows users to connect ChatGPT even without a subscription, then see the
	// subscription requirement when they try to use features in chat

	// User is authenticated - check if they've already authorized this client
	// For now, we auto-approve (can add consent screen later)
	// In production, you might want to:
	// 1. Check if user has previously authorized this client
	// 2. Show a consent screen for first-time authorization
	// 3. Store user's consent preferences

	// Generate authorization code
	authCode := uuid.New().String()

	// Store authorization code in database (for PKCE verification)
	// Expires in 10 minutes - use UTC to match PostgreSQL's NOW()
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	slog.InfoContext(r.Context(), "mcp auth: creating authorization code", "expires_at", expiresAt.Format(time.RFC3339))
	result, err := h.db.Pool.Exec(r.Context(), `
		INSERT INTO oauth_authorization_codes (code, user_id, client_id, redirect_uri, code_challenge, code_challenge_method, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, authCode, user.ID, clientID, redirectURI, codeChallenge, "S256", scopes, expiresAt)

	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to save authorization code", "err", err)
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	rowsAffected := result.RowsAffected()
	slog.InfoContext(r.Context(), "mcp auth: saved authorization code", "code_prefix", authCode[:min(8, len(authCode))], "user_id", user.ID, "client_id", clientID, "expires_at", expiresAt.Format(time.RFC3339), "rows_affected", rowsAffected)

	// Redirect back to ChatGPT with authorization code
	redirectURL, _ := url.Parse(redirectURI)
	q := redirectURL.Query()
	q.Set("code", authCode)
	if state != "" {
		q.Set("state", state)
	}
	redirectURL.RawQuery = q.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// HandleCallback handles OAuth callback (for internal use, ChatGPT handles the redirect)
// POST /mcp/callback (token exchange)
func (h *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// This endpoint handles token exchange (not the redirect callback)
	// ChatGPT will POST here to exchange authorization code for access token

	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GrantType    string `json:"grant_type"`
		Code         string `json:"code"` // Authorization code OR refresh_token (depending on grant_type)
		RedirectURI  string `json:"redirect_uri"`
		ClientID     string `json:"client_id"`
		CodeVerifier string `json:"code_verifier"` // PKCE
		RefreshToken string `json:"refresh_token"` // For refresh_token grant type
	}

	// Read body for debugging and parsing
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// OAuth 2.1 token endpoint accepts both JSON and form-encoded requests
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		// Parse as form data
		if err := r.ParseForm(); err != nil {
			slog.ErrorContext(r.Context(), "Failed to parse form", "err", err)
			http.Error(w, "invalid_request", http.StatusBadRequest)
			return
		}
		req.GrantType = r.FormValue("grant_type")
		req.Code = r.FormValue("code")
		req.RedirectURI = r.FormValue("redirect_uri")
		req.ClientID = r.FormValue("client_id")
		req.CodeVerifier = r.FormValue("code_verifier")
		req.RefreshToken = r.FormValue("refresh_token")
	} else {
		// Parse as JSON
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.ErrorContext(r.Context(), "failed to parse JSON body", "err", err, "body", string(bodyBytes))
			http.Error(w, "invalid_request", http.StatusBadRequest)
			return
		}
	}

	slog.InfoContext(r.Context(), "mcp callback: token exchange request", "grant_type", req.GrantType, "code_prefix", req.Code[:min(8, len(req.Code))], "client_id", req.ClientID, "redirect_uri", req.RedirectURI, "has_code_verifier", req.CodeVerifier != "")

	// Handle different grant types
	if req.GrantType == "refresh_token" {
		// Handle refresh token grant
		refreshToken := req.RefreshToken
		if refreshToken == "" {
			// Fallback: ChatGPT might use 'code' field for refresh_token
			refreshToken = req.Code
		}
		if refreshToken == "" {
			slog.WarnContext(r.Context(), "Missing refresh_token")
			http.Error(w, "invalid_request: refresh_token required", http.StatusBadRequest)
			return
		}

		slog.InfoContext(r.Context(), "mcp callback: refresh token request", "token_prefix", refreshToken[:min(8, len(refreshToken))]+"...", "client_id", req.ClientID)

		// Validate refresh token and create new access token
		var token oauth.Token
		var scopes []string
		err := h.db.Pool.QueryRow(r.Context(), `
			SELECT id, user_id, client_id, scopes, access_token, expires_at, created_at
			FROM oauth_tokens
			WHERE refresh_token = $1
		`, refreshToken).Scan(&token.ID, &token.UserID, &token.ClientID, &scopes, &token.AccessToken, &token.ExpiresAt, &token.CreatedAt)

		if err != nil {
			slog.WarnContext(r.Context(), "Invalid refresh_token", "err", err)
			http.Error(w, "invalid_grant: refresh token invalid or expired", http.StatusBadRequest)
			return
		}

		// Create new access token
		newToken, err := h.tokenStore.CreateToken(r.Context(), token.UserID, req.ClientID, scopes, 3600)
		if err != nil {
			slog.ErrorContext(r.Context(), "Failed to create new token", "err", err)
			http.Error(w, "server_error", http.StatusInternalServerError)
			return
		}

		// Return new token
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  newToken.AccessToken,
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": newToken.RefreshToken,
			"scope":         strings.Join(scopes, " "),
		})
		return
	}

	// Validate grant type for authorization code flow
	if req.GrantType != "authorization_code" {
		slog.WarnContext(r.Context(), "invalid grant_type", "grant_type", req.GrantType)
		http.Error(w, "unsupported_grant_type", http.StatusBadRequest)
		return
	}

	// Get authorization code from database
	var authCode struct {
		UserID              uuid.UUID
		ClientID            string
		RedirectURI         string
		CodeChallenge       string
		CodeChallengeMethod string
		Scopes              []string
		ExpiresAt           time.Time
	}

	// Query with UTC comparison to handle both not-found and expired cases
	err := h.db.Pool.QueryRow(r.Context(), `
		SELECT user_id, client_id, redirect_uri, code_challenge, code_challenge_method, scopes, expires_at
		FROM oauth_authorization_codes
		WHERE code = $1 AND expires_at > (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
	`, req.Code).Scan(
		&authCode.UserID, &authCode.ClientID, &authCode.RedirectURI,
		&authCode.CodeChallenge, &authCode.CodeChallengeMethod, &authCode.Scopes, &authCode.ExpiresAt,
	)

	if err != nil {
		// Check if it's expired
		var expiresAt time.Time
		expiredErr := h.db.Pool.QueryRow(r.Context(), `
			SELECT expires_at FROM oauth_authorization_codes WHERE code = $1
		`, req.Code).Scan(&expiresAt)
		if expiredErr == nil {
			nowUTC := time.Now().UTC()
			slog.WarnContext(r.Context(), "mcp callback: authorization code expired", "code_prefix", req.Code[:min(8, len(req.Code))], "expires_at", expiresAt.Format(time.RFC3339), "now", nowUTC.Format(time.RFC3339), "expired", nowUTC.After(expiresAt))
		} else {
			slog.ErrorContext(r.Context(), "authorization code not found or expired", "code_prefix", req.Code[:min(8, len(req.Code))]+"...", "err", err)
		}
		http.Error(w, "invalid_grant: authorization code not found or expired", http.StatusBadRequest)
		return
	}

	slog.InfoContext(r.Context(), "found authorization code", "client_id", authCode.ClientID, "redirect_uri", authCode.RedirectURI)

	// Verify PKCE code verifier
	expectedChallenge := generateCodeChallengeS256(req.CodeVerifier)
	if expectedChallenge != authCode.CodeChallenge {
		slog.ErrorContext(r.Context(), "PKCE verification failed", "expected", authCode.CodeChallenge[:min(16, len(authCode.CodeChallenge))]+"...", "got", expectedChallenge[:min(16, len(expectedChallenge))]+"...")
		http.Error(w, "invalid_grant: PKCE verification failed", http.StatusBadRequest)
		return
	}

	// Verify redirect URI matches
	if req.RedirectURI != authCode.RedirectURI {
		slog.WarnContext(r.Context(), "redirect URI mismatch", "expected", authCode.RedirectURI, "got", req.RedirectURI)
		http.Error(w, "invalid_grant: redirect_uri mismatch", http.StatusBadRequest)
		return
	}

	// Verify client ID matches
	if req.ClientID != authCode.ClientID {
		slog.WarnContext(r.Context(), "client_id mismatch", "expected", authCode.ClientID, "got", req.ClientID)
		http.Error(w, "invalid_grant: client_id mismatch", http.StatusBadRequest)
		return
	}

	// Delete authorization code (one-time use)
	if _, err := h.db.Pool.Exec(r.Context(), `DELETE FROM oauth_authorization_codes WHERE code = $1`, req.Code); err != nil {
		slog.WarnContext(r.Context(), "failed to delete authorization code", "err", err)
	}

	userID := authCode.UserID
	scopes := authCode.Scopes

	// Create access token
	token, err := h.tokenStore.CreateToken(r.Context(), userID, req.ClientID, scopes, 3600) // 1 hour expiry
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	// Return token response
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  token.AccessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": token.RefreshToken,
		"scope":         strings.Join(scopes, " "),
	})
}

// HandleMetadata returns OAuth server metadata for discovery
// GET /.well-known/oauth-protected-resource
func (h *AuthHandler) HandleMetadata(w http.ResponseWriter, r *http.Request) {
	baseURL := h.cfg.MCPBaseURL
	if baseURL == "" {
		baseURL = h.cfg.BaseURL
	}

	metadata := map[string]interface{}{
		"issuer":                           baseURL,
		"authorization_endpoint":           fmt.Sprintf("%s/mcp/auth", baseURL),
		"token_endpoint":                   fmt.Sprintf("%s/mcp/callback", baseURL),
		"registration_endpoint":            fmt.Sprintf("%s/mcp/register", baseURL),
		"code_challenge_methods_supported": []string{"S256"},
		"scopes_supported":                 supportedScopes(),
		"response_types_supported": []string{"code"},
		"grant_types_supported":    []string{"authorization_code"},
		// Resource server metadata (RFC 7662)
		"bearer_methods_supported": []string{"bearer"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metadata)
}

// supportedScopes is the single source of truth for the OAuth scopes the server
// grants. Both OAuth metadata documents and the Claude client config read from
// it so the published scope list can never drift between endpoints. A fresh
// slice is returned each call to keep callers from mutating shared state.
func supportedScopes() []string {
	return []string{
		"read:transactions",
		"read:accounts",
		"read:financial",
		"read:patterns",
	}
}

// HandleAuthorizationServerMetadata returns OAuth authorization server metadata (RFC 8414)
// GET /.well-known/oauth-authorization-server
func (h *AuthHandler) HandleAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	baseURL := h.cfg.MCPBaseURL
	if baseURL == "" {
		baseURL = h.cfg.BaseURL
	}

	metadata := map[string]interface{}{
		"issuer":                           baseURL,
		"authorization_endpoint":           fmt.Sprintf("%s/mcp/auth", baseURL),
		"token_endpoint":                   fmt.Sprintf("%s/mcp/callback", baseURL),
		"registration_endpoint":            fmt.Sprintf("%s/mcp/register", baseURL),
		"code_challenge_methods_supported": []string{"S256"},
		"scopes_supported":                 supportedScopes(),
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"token_endpoint_auth_methods_supported": []string{"none"}, // Public client with PKCE
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metadata)
}

// HandleClientRegistration handles Dynamic Client Registration
// POST /mcp/register
func (h *AuthHandler) HandleClientRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RedirectURIs []string `json:"redirect_uris"`
		Scopes       []string `json:"scopes,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}

	// Validate redirect URIs (must be HTTPS in production)
	for _, uri := range req.RedirectURIs {
		parsed, err := url.Parse(uri)
		if err != nil {
			http.Error(w, "invalid_redirect_uri", http.StatusBadRequest)
			return
		}
		if h.cfg.Environment == "production" && parsed.Scheme != "https" {
			http.Error(w, "invalid_redirect_uri: must use HTTPS in production", http.StatusBadRequest)
			return
		}
	}

	// Register client
	client, err := h.clientStore.RegisterClient(r.Context(), req.RedirectURIs, req.Scopes)
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	// Return client credentials
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"client_id":                  client.ClientID,
		"client_secret":              client.ClientSecret, // Empty for public clients
		"redirect_uris":              client.RedirectURIs,
		"scope":                      strings.Join(client.Scopes, " "),
		"client_id_issued_at":        client.CreatedAt.Unix(),
		"token_endpoint_auth_method": "none", // Public client with PKCE
	})
}

// getUserFromSession extracts user from session using Authboss
// For ChatGPT Apps, users must be logged into Probably first
func (h *AuthHandler) getUserFromSession(r *http.Request) *models.User {
	// Check if Authboss is available (may be nil if type assertion failed during initialization)
	if h.ab == nil {
		return nil
	}

	// Use Authboss to get current user ID (PID = email)
	pid, err := h.ab.CurrentUserID(r)
	if err != nil || pid == "" {
		return nil
	}

	// Load full user from database
	user, err := h.userStore.GetByEmail(r.Context(), pid)
	if err != nil {
		return nil
	}

	return user
}

// generateCodeChallengeS256 generates a PKCE code challenge using S256
func generateCodeChallengeS256(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
