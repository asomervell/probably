package auth

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/aarondl/authboss/v3"
	"github.com/asomervell/probably/internal/views/pages"
)

// Logger implements authboss.Logger
type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

func (l *Logger) Info(s string)  { slog.Info(s, "component", "authboss") }
func (l *Logger) Error(s string) { slog.Error(s, "component", "authboss") }

// ViewRenderer renders authboss pages using gomponents
type ViewRenderer struct{}

func NewViewRenderer() *ViewRenderer {
	return &ViewRenderer{}
}

func (v *ViewRenderer) Load(names ...string) error {
	return nil
}

func (v *ViewRenderer) Render(ctx context.Context, page string, data authboss.HTMLData) ([]byte, string, error) {
	var content []byte
	var err error

	// Extract redirect parameter from request context and add to data
	// This preserves the redirect through the login POST
	// Try multiple ways to get the request from context
	var req *http.Request
	if r, ok := ctx.Value("request").(*http.Request); ok {
		req = r
	} else if r, ok := ctx.Value(http.Request{}).(*http.Request); ok {
		req = r
	} else if r := ctx.Value("http_request"); r != nil {
		if httpReq, ok := r.(*http.Request); ok {
			req = httpReq
		}
	}

	if req != nil {
		redirect := req.URL.Query().Get("redirect")
		if redirect == "" {
			redirect = req.URL.Query().Get("redir")
		}
		if redirect != "" {
			data["redirect"] = redirect
		}
	}

	switch page {
	case "login":
		content, err = pages.RenderLogin(data)
	case "register":
		content, err = pages.RenderRegister(data)
	case "recover_start":
		content, err = pages.RenderRecoverStart(data)
	case "recover_end":
		content, err = pages.RenderRecoverEnd(data)
	default:
		content, err = pages.RenderLogin(data)
	}

	if err != nil {
		return nil, "", err
	}

	return content, "text/html", nil
}

// MailRenderer renders email templates
type MailRenderer struct{}

func NewMailRenderer() *MailRenderer {
	return &MailRenderer{}
}

func (m *MailRenderer) Load(names ...string) error {
	return nil
}

func (m *MailRenderer) Render(ctx context.Context, page string, data authboss.HTMLData) ([]byte, string, error) {
	var content string

	switch page {
	case "recover_html":
		url, _ := data["recover_url"].(string)
		content = `<html><body>
			<p>Click the link below to reset your password:</p>
			<p><a href="` + url + `">Reset Password</a></p>
		</body></html>`
	case "recover_txt":
		url, _ := data["recover_url"].(string)
		content = "Click the link below to reset your password:\n\n" + url
	case "confirm_html":
		url, _ := data["confirm_url"].(string)
		content = `<html><body>
			<p>Click the link below to confirm your email:</p>
			<p><a href="` + url + `">Confirm Email</a></p>
		</body></html>`
	case "confirm_txt":
		url, _ := data["confirm_url"].(string)
		content = "Click the link below to confirm your email:\n\n" + url
	}

	return []byte(content), "text/html", nil
}

// ConsoleMailer prints emails to console (for development)
type ConsoleMailer struct{}

func NewConsoleMailer() *ConsoleMailer {
	return &ConsoleMailer{}
}

func (c *ConsoleMailer) Send(ctx context.Context, email authboss.Email) error {
	println("===== EMAIL =====")
	println("To:", email.To[0])
	println("Subject:", email.Subject)
	println("Body:", email.TextBody)
	println("=================")
	return nil
}

// Responder handles HTTP responses for authboss
type Responder struct {
	renderer *ViewRenderer
}

func NewResponder(renderer *ViewRenderer) *Responder {
	return &Responder{renderer: renderer}
}

func (r *Responder) Respond(w http.ResponseWriter, req *http.Request, code int, templateName string, data authboss.HTMLData) error {
	content, contentType, err := r.renderer.Render(req.Context(), templateName, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(code)
	_, err = w.Write(content)
	return err
}

// Redirector handles redirects for authboss
type Redirector struct{}

func NewRedirector() *Redirector {
	return &Redirector{}
}

func (r *Redirector) Redirect(w http.ResponseWriter, req *http.Request, ro authboss.RedirectOptions) error {
	path := ro.RedirectPath

	// Always check for redirect parameter (for OAuth flow support)
	// Check both "redir" (Authboss standard) and "redirect" (OAuth flow)
	// Check form values first (for POST requests), then query string
	// This is important because after login POST, the redirect is in the form body
	redir := req.FormValue("redirect")
	if redir == "" {
		redir = req.FormValue("redir")
	}
	if redir == "" {
		redir = req.URL.Query().Get("redirect")
	}
	if redir == "" {
		redir = req.URL.Query().Get("redir")
	}
	if redir != "" {
		// URL decode the redirect parameter (it might be double-encoded)
		// Try decoding multiple times in case it's nested
		decoded := redir
		for i := 0; i < 3; i++ {
			if d, err := url.QueryUnescape(decoded); err == nil && d != decoded {
				decoded = d
			} else {
				break
			}
		}
		redir = decoded

		// Validate redirect is a safe internal path (prevent open redirect)
		// Parse the URL to check only the path, not query parameters
		// Query parameters may contain external URLs (like OAuth redirect_uri) which is safe
		if parsedURL, err := url.Parse(redir); err == nil {
			// Check that the path starts with / and doesn't contain :// in the path itself
			// Query parameters are allowed to contain external URLs
			if strings.HasPrefix(parsedURL.Path, "/") && !strings.Contains(parsedURL.Path, "://") {
				path = redir
			} else {
				slog.WarnContext(req.Context(), "rejected redirect (invalid path)", "path", parsedURL.Path)
			}
		} else {
			// If parsing fails, fall back to simple check (shouldn't happen with valid URLs)
			if strings.HasPrefix(redir, "/") {
				// Only check path part before query string
				pathPart := strings.Split(redir, "?")[0]
				if !strings.Contains(pathPart, "://") {
					path = redir
				} else {
					slog.WarnContext(req.Context(), "rejected redirect (contains :// in path)", "path", pathPart)
				}
			} else {
				slog.WarnContext(req.Context(), "rejected redirect (doesn't start with /)", "redirect", redir)
			}
		}
		// Ignore external URLs to prevent open redirect attacks
	} else {
		slog.InfoContext(req.Context(), "no redirect parameter found, using default", "redirect", path)
	}

	// Always use 303 See Other for POST requests to ensure the redirect uses GET
	// This prevents 405 errors when redirecting to routes that only accept GET
	code := ro.Code
	if code == 0 || req.Method == http.MethodPost {
		code = http.StatusSeeOther // 303 - always converts to GET
	}

	http.Redirect(w, req, path, code)
	return nil
}

// Router implements authboss.Router
type Router struct {
	mux *http.ServeMux
}

func NewRouter() *Router {
	return &Router{mux: http.NewServeMux()}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (r *Router) Get(path string, handler http.Handler) {
	r.mux.Handle("GET "+path, handler)
}

func (r *Router) Post(path string, handler http.Handler) {
	r.mux.Handle("POST "+path, handler)
}

func (r *Router) Delete(path string, handler http.Handler) {
	r.mux.Handle("DELETE "+path, handler)
}

// ErrorHandler implements authboss.ErrorHandler
type ErrorHandler struct{}

func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{}
}

func (e *ErrorHandler) Wrap(fn func(w http.ResponseWriter, r *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

// BodyReader implements authboss.BodyReader
type BodyReader struct{}

func NewBodyReader() *BodyReader {
	return &BodyReader{}
}

func (b *BodyReader) Read(page string, r *http.Request) (authboss.Validator, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	switch page {
	case "login":
		return &loginValidator{
			Email:    r.FormValue("email"),
			Password: r.FormValue("password"),
		}, nil
	case "register":
		return &registerValidator{
			Email:           r.FormValue("email"),
			Password:        r.FormValue("password"),
			ConfirmPassword: r.FormValue("confirm_password"),
		}, nil
	case "recover_start":
		return &recoverStartValidator{
			Email: r.FormValue("email"),
		}, nil
	case "recover_end":
		return &recoverEndValidator{
			Password:        r.FormValue("password"),
			ConfirmPassword: r.FormValue("confirm_password"),
			Token:           r.FormValue("token"),
		}, nil
	default:
		return nil, nil
	}
}

// Validators for form data

type fieldError struct {
	name string
	err  string
}

func (f fieldError) Name() string  { return f.name }
func (f fieldError) Err() error    { return nil }
func (f fieldError) Error() string { return f.err }

type loginValidator struct {
	Email    string
	Password string
}

func (l *loginValidator) Validate() []error {
	return nil
}

func (l *loginValidator) GetPID() string {
	return l.Email
}

func (l *loginValidator) GetPassword() string {
	return l.Password
}

type registerValidator struct {
	Email           string
	Password        string
	ConfirmPassword string
}

func (r *registerValidator) Validate() []error {
	var errs []error
	if r.Password != r.ConfirmPassword {
		errs = append(errs, fieldError{name: "confirm_password", err: "Passwords do not match"})
	}
	return errs
}

func (r *registerValidator) GetPID() string {
	return r.Email
}

func (r *registerValidator) GetPassword() string {
	return r.Password
}

type recoverStartValidator struct {
	Email string
}

func (r *recoverStartValidator) Validate() []error {
	return nil
}

func (r *recoverStartValidator) GetPID() string {
	return r.Email
}

type recoverEndValidator struct {
	Password        string
	ConfirmPassword string
	Token           string
}

func (r *recoverEndValidator) Validate() []error {
	var errs []error
	if r.Password != r.ConfirmPassword {
		errs = append(errs, fieldError{name: "confirm_password", err: "Passwords do not match"})
	}
	return errs
}

func (r *recoverEndValidator) GetPassword() string {
	return r.Password
}

func (r *recoverEndValidator) GetToken() string {
	return r.Token
}
