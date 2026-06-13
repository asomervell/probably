package mcp

import (
	"context"
	"net/http"
	"strings"

	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/oauth"
	"github.com/google/uuid"
)

// UserContext holds user and ledger information for MCP requests
type UserContext struct {
	UserID   uuid.UUID
	LedgerID uuid.UUID
	APIKey   string
	Scopes   []string // OAuth scopes granted
}

type contextKey string

const userContextKey contextKey = "mcp_user_context"

// ContextHandler manages user context extraction from OAuth tokens
type ContextHandler struct {
	tokenStore  *oauth.TokenStore
	userStore   *models.UserStore
	ledgerStore *models.LedgerStore
}

// NewContextHandler creates a new context handler
func NewContextHandler(database *db.DB, tokenStore *oauth.TokenStore) *ContextHandler {
	return &ContextHandler{
		tokenStore:  tokenStore,
		userStore:   models.NewUserStore(database.Pool),
		ledgerStore: models.NewLedgerStore(database.Pool),
	}
}

// Middleware extracts user context from OAuth access token
// Validates OAuth tokens according to OAuth 2.1 specification
func (h *ContextHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeBearerError(w, `Bearer error="missing_token"`, `{"error": "missing Authorization header"}`)
			return
		}

		// Expect "Bearer <oauth_token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			writeBearerError(w, `Bearer error="invalid_token"`, `{"error": "invalid Authorization header format"}`)
			return
		}

		accessToken := parts[1]
		if accessToken == "" {
			writeBearerError(w, `Bearer error="invalid_token"`, `{"error": "empty token"}`)
			return
		}

		// Validate OAuth access token
		oauthToken, err := h.tokenStore.ValidateToken(r.Context(), accessToken)
		if err != nil {
			writeBearerError(w, `Bearer error="invalid_token", error_description="Token expired or invalid"`, `{"error": "invalid or expired token"}`)
			return
		}

		// Load user
		user, err := h.userStore.GetByID(r.Context(), oauthToken.UserID)
		if err != nil {
			writeBearerError(w, `Bearer error="invalid_token"`, `{"error": "user not found"}`)
			return
		}

		// Get user's default ledger
		ledger, err := h.ledgerStore.GetByUserID(r.Context(), user.ID)
		if err != nil {
			writeBearerError(w, `Bearer error="invalid_token"`, `{"error": "no ledger found"}`)
			return
		}

		// Create user context with OAuth token info
		userCtx := &UserContext{
			UserID:   user.ID,
			LedgerID: ledger.ID,
			APIKey:   oauthToken.APIKey,
			Scopes:   oauthToken.Scopes,
		}

		// Add to request context
		ctx := context.WithValue(r.Context(), userContextKey, userCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetContext extracts user context from request context
func (h *ContextHandler) GetContext(ctx context.Context) *UserContext {
	userCtx, ok := ctx.Value(userContextKey).(*UserContext)
	if !ok {
		return nil
	}
	return userCtx
}

// writeBearerError writes a 401 Unauthorized response with a WWW-Authenticate Bearer error header.
func writeBearerError(w http.ResponseWriter, wwwAuthenticate, jsonBody string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", wwwAuthenticate)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(jsonBody))
}
