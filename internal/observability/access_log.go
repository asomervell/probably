package observability

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// SlogAccessMiddleware logs each completed request with structured slog (forwards to PostHog OTLP when enabled).
// On status >= 500, also sends a PostHog capture for alerting. Place after RequestID and with distinct_id on context.
func SlogAccessMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t0 := time.Now()
			defer func() {
				dur := time.Since(t0)
				status := ww.Status()
				if status == 0 {
					status = 200
				}
				reqID := middleware.GetReqID(r.Context())
				routePattern := r.URL.Path
				if rc := chi.RouteContext(r.Context()); rc != nil {
					if p := rc.RoutePattern(); p != "" {
						routePattern = p
					}
				}
				attrs := []slog.Attr{
					slog.String("http.method", r.Method),
					slog.String("http.path", r.URL.Path),
					slog.String("http.route", routePattern),
					slog.Int("http.status", status),
					slog.Int("http.response.size", ww.BytesWritten()),
					slog.Float64("http.duration_ms", float64(dur.Milliseconds())),
				}
				if reqID != "" {
					attrs = append(attrs, slog.String("http.request_id", reqID))
				}
				if rq := r.URL.RawQuery; rq != "" {
					attrs = append(attrs, slog.String("http.query", rq))
				}

				var lvl slog.Level = slog.LevelInfo
				if status >= 500 {
					lvl = slog.LevelError
				}

				slog.LogAttrs(r.Context(), lvl, "http request", attrs...)

				if status >= 500 {
					err := fmt.Errorf("http %d %s %s", status, r.Method, r.URL.Path)
					CaptureFailure(r.Context(), err, FailureOptions{
						Component: "http",
						Operation: "request",
						Tags: map[string]string{
							"http.status":     fmt.Sprintf("%d", status),
							"http.method":     r.Method,
							"http.path":       r.URL.Path,
							"http.route":      routePattern,
							"http.request_id": reqID,
						},
					})
				}
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
