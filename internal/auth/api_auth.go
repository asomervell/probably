package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/aarondl/authboss/v3"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
)

// Context keys for API auth
type contextKey string

const (
	APIKeyContextKey  contextKey = "api_key"
	APIUserContextKey contextKey = "api_user"
)

// APIKeyMiddleware validates API keys passed via `Authorization: Bearer <api_key>`
func APIKeyMiddleware(apiKeyStore *models.APIKeyStore, userStore *models.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error": "missing Authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, `{"error": "invalid Authorization header format, expected 'Bearer <api_key>'"}`, http.StatusUnauthorized)
				return
			}

			apiKeyPlaintext := strings.TrimSpace(parts[1])
			if apiKeyPlaintext == "" {
				http.Error(w, `{"error": "empty API key"}`, http.StatusUnauthorized)
				return
			}

			apiKey, err := apiKeyStore.ValidateAndGetUser(r.Context(), apiKeyPlaintext)
			if err != nil {
				http.Error(w, `{"error": "invalid API key"}`, http.StatusUnauthorized)
				return
			}

			user, err := userStore.GetByID(r.Context(), apiKey.UserID)
			if err != nil {
				http.Error(w, `{"error": "user not found"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), APIKeyContextKey, apiKey)
			ctx = withAuthbossUser(ctx, user)
			ctx = observability.WithDistinctID(ctx, user.ID.String())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APICurrentUser extracts the current user from context (set by APIKeyMiddleware)
func APICurrentUser(r *http.Request) *models.User {
	user, ok := r.Context().Value(APIUserContextKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}

func withAuthbossUser(ctx context.Context, user *models.User) context.Context {
	if user == nil {
		return ctx
	}
	ctx = context.WithValue(ctx, APIUserContextKey, user)
	ctx = context.WithValue(ctx, authboss.CTXKeyUser, user)
	ctx = context.WithValue(ctx, authboss.CTXKeyPID, user.Email)
	return ctx
}
