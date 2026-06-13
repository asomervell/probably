package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/aarondl/authboss-clientstate"
	"github.com/aarondl/authboss/v3"
	_ "github.com/aarondl/authboss/v3/auth"
	_ "github.com/aarondl/authboss/v3/logout"
	_ "github.com/aarondl/authboss/v3/recover"
	_ "github.com/aarondl/authboss/v3/register"
	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/db"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/observability"
	"github.com/gorilla/sessions"
)

// SetupAuthboss initializes and configures Authboss
func SetupAuthboss(cfg *config.Config, database *db.DB) (*authboss.Authboss, *sessions.CookieStore, error) {
	ab := authboss.New()

	// Cookie store for sessions
	sessionStore := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	sessionStore.Options.HttpOnly = true
	sessionStore.Options.Secure = cfg.Environment == "production"
	sessionStore.Options.SameSite = http.SameSiteLaxMode
	sessionStore.Options.MaxAge = 60 * 60 * 24 * 30 // 30 days

	cookieStorer := abclientstate.NewCookieStorer([]byte(cfg.SessionSecret), nil)
	cookieStorer.HTTPOnly = true
	cookieStorer.Secure = cfg.Environment == "production"

	sessionStorer := abclientstate.NewSessionStorerFromExisting("probably_session", sessionStore)

	// Configure Authboss
	ab.Config.Paths.Mount = "/auth"
	ab.Config.Paths.RootURL = cfg.BaseURL
	// Default redirect after login - but can be overridden by redirect parameter
	ab.Config.Paths.AuthLoginOK = "/intelligence"
	ab.Config.Paths.LogoutOK = "/"
	ab.Config.Paths.RegisterOK = "/intelligence"
	ab.Config.Paths.RecoverOK = "/auth/login"

	ab.Config.Core.ViewRenderer = NewViewRenderer()
	ab.Config.Core.MailRenderer = NewMailRenderer()
	ab.Config.Core.Responder = NewResponder(NewViewRenderer())
	ab.Config.Core.Redirector = NewRedirector()
	ab.Config.Core.Router = NewRouter()
	ab.Config.Core.ErrorHandler = NewErrorHandler()
	ab.Config.Core.BodyReader = NewBodyReader()
	ab.Config.Core.Logger = NewLogger()

	// Mailer (console for now)
	ab.Config.Core.Mailer = NewConsoleMailer()

	// Storage
	userStore := models.NewUserStore(database.Pool)
	ab.Config.Storage.Server = NewStorer(userStore)
	ab.Config.Storage.SessionState = sessionStorer
	ab.Config.Storage.CookieState = cookieStorer

	// Modules to load
	ab.Config.Modules.LogoutMethod = "POST"
	ab.Config.Modules.RegisterPreserveFields = []string{"email"}
	ab.Config.Modules.RecoverTokenDuration = 24 * time.Hour
	ab.Config.Modules.RecoverLoginAfterRecovery = false

	// Initialize
	if err := ab.Init(); err != nil {
		return nil, nil, err
	}

	return ab, sessionStore, nil
}

// CurrentUser extracts the current user from context
func CurrentUser(r *http.Request) *models.User {
	user, ok := r.Context().Value(authboss.CTXKeyUser).(*models.User)
	if !ok {
		return nil
	}
	return user
}

// AnyCurrentUser returns the session user, falling back to the API-key user.
// Use in handlers that accept both browser sessions and API key auth.
func AnyCurrentUser(r *http.Request) *models.User {
	if u := CurrentUser(r); u != nil {
		return u
	}
	return APICurrentUser(r)
}

// CurrentUserID extracts the current user's PID (email) from context
func CurrentUserID(r *http.Request) string {
	pid, ok := r.Context().Value(authboss.CTXKeyPID).(string)
	if !ok {
		return ""
	}
	return pid
}

// RequireAuth is middleware that requires authentication
func RequireAuth(ab *authboss.Authboss) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if CurrentUserID(r) == "" {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LoadUserMiddleware loads the user into context
func LoadUserMiddleware(ab *authboss.Authboss, userStore *models.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user ID from authboss
			pid, err := ab.CurrentUserID(r)
			if err != nil || pid == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Load full user
			user, err := userStore.GetByEmail(r.Context(), pid)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Add to context
			ctx := context.WithValue(r.Context(), authboss.CTXKeyPID, pid)
			ctx = context.WithValue(ctx, authboss.CTXKeyUser, user)
			ctx = observability.WithDistinctID(ctx, user.ID.String())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
