package auth

import (
	"net/http"
	"strings"

	"github.com/asomervell/probably/internal/models"
)

// RequireSubscriptionOrTrial middleware requires users to have an active subscription or trial
func RequireSubscriptionOrTrial(subscriptionStore *models.SubscriptionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := CurrentUser(r)
			if user == nil {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			// Skip subscription check for OAuth/MCP endpoints - allow connection without subscription
			// Subscription will be checked when tools are actually used
			if strings.HasPrefix(r.URL.Path, "/mcp/") {
				next.ServeHTTP(w, r)
				return
			}

			// Check if user has active subscription or trial
			hasAccess, err := subscriptionStore.HasActiveSubscriptionOrTrial(r.Context(), user.ID)
			if err != nil {
				// On error, allow access but log it (better UX than blocking)
				http.Error(w, "Failed to check subscription status", http.StatusInternalServerError)
				return
			}

			if !hasAccess {
				// Redirect to billing page with message
				http.Redirect(w, r, "/settings/billing?error=subscription_required", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
