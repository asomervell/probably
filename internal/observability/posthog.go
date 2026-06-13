package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/posthog/posthog-go"
)

type contextKey string

const (
	distinctIDKey contextKey = "posthog_distinct_id"
)

var (
	phClient      posthog.Client
	phClientMu    sync.RWMutex
	phService     string
	phEnvironment string
	phEnabled     bool
)

// InitPostHog configures the global PostHog client (events, exceptions, flags).
func InitPostHog(cfg *config.Config, serviceName string) {
	phClientMu.Lock()
	defer phClientMu.Unlock()

	if phClient != nil {
		_ = phClient.Close()
		phClient = nil
	}

	phService = serviceName
	phEnvironment = cfg.PostHogEnvironment
	if phEnvironment == "" {
		phEnvironment = cfg.Environment
	}

	if cfg.PostHogProjectAPIKey == "" {
		phEnabled = false
		slog.Info("PostHog capture disabled; POSTHOG_PROJECT_API_KEY not set")
		return
	}

	endpoint := cfg.PostHogAPIHost
	if endpoint == "" {
		endpoint = "https://us.i.posthog.com"
	}

	phCfg := posthog.Config{
		Endpoint:                           endpoint,
		PersonalApiKey:                     cfg.PostHogPersonalAPIKey,
		DefaultFeatureFlagsPollingInterval: 300 * time.Second,
	}
	// Shorter flush in dev so events/exceptions show up quickly in PostHog.
	if cfg.Environment == "development" {
		phCfg.Interval = 1 * time.Second
	}
	pc, err := posthog.NewWithConfig(cfg.PostHogProjectAPIKey, phCfg)
	if err != nil {
		slog.Error("PostHog client init failed", "error", err)
		phEnabled = false
		return
	}
	phClient = pc
	phEnabled = true
	slog.Info("PostHog client initialized", "service", serviceName, "env", phEnvironment)
}

// PostHogEnabled reports whether the capture client is active.
func PostHogEnabled() bool {
	phClientMu.RLock()
	defer phClientMu.RUnlock()
	return phEnabled && phClient != nil
}

func client() posthog.Client {
	phClientMu.RLock()
	defer phClientMu.RUnlock()
	return phClient
}

// ClosePostHog flushes and closes the PostHog client.
func ClosePostHog() {
	phClientMu.Lock()
	defer phClientMu.Unlock()
	if phClient != nil {
		if err := phClient.Close(); err != nil {
			slog.Error("PostHog client close error", "error", err)
		}
		phClient = nil
	}
	phEnabled = false
}

// WithDistinctID returns a child context carrying the PostHog distinct_id for captures.
func WithDistinctID(ctx context.Context, distinctID string) context.Context {
	if distinctID == "" {
		return ctx
	}
	return context.WithValue(ctx, distinctIDKey, distinctID)
}

// DistinctIDFromContext returns the distinct id from context, or "anonymous".
func DistinctIDFromContext(ctx context.Context) string {
	if v := ctx.Value(distinctIDKey); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "anonymous"
}

// captureError reports an error to PostHog as an exception event.
func captureError(ctx context.Context, err error, tags map[string]string) {
	if err == nil {
		return
	}
	c := client()
	if c == nil {
		return
	}
	did := DistinctIDFromContext(ctx)
	props := posthog.NewProperties().
		Set("environment", phEnvironment).
		Set("service", phService)
	for k, v := range tags {
		props = props.Set(k, v)
	}
	tracer := posthog.DefaultStackTraceExtractor{InAppDecider: posthog.SimpleInAppDecider}
	_ = c.Enqueue(posthog.Exception{
		DistinctId: did,
		Timestamp:  time.Now(),
		Properties: props,
		ExceptionList: []posthog.ExceptionItem{{
			Type:       errorTypeName(err),
			Value:      err.Error(),
			Stacktrace: tracer.GetStackTrace(3),
		}},
	})
}

// errorTypeName returns the concrete Go type name of err for PostHog exception tracking.
// Skips standard-library wrapper packages (fmt, errors) so that the innermost meaningful
// type (e.g. TellerAPIError, PgError) is used as the exception title rather than a generic name.
func errorTypeName(err error) string {
	for err != nil {
		t := reflect.TypeOf(err)
		if t == nil {
			return "Error"
		}
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if pkg := t.PkgPath(); pkg != "fmt" && pkg != "errors" {
			if name := t.Name(); name != "" {
				return name
			}
		}
		err = errors.Unwrap(err)
	}
	return "Error"
}

// CaptureEvent sends a named analytics event to PostHog with arbitrary properties.
func CaptureEvent(ctx context.Context, event string, properties map[string]any) {
	c := client()
	if c == nil || event == "" {
		return
	}
	props := posthog.NewProperties().
		Set("environment", phEnvironment).
		Set("service", phService)
	for k, v := range properties {
		props = props.Set(k, v)
	}
	_ = c.Enqueue(posthog.Capture{
		DistinctId: DistinctIDFromContext(ctx),
		Event:      event,
		Properties: props,
	})
}

// CaptureMessage sends a structured message to PostHog.
func CaptureMessage(ctx context.Context, message string, tags map[string]string) {
	c := client()
	if c == nil || message == "" {
		return
	}
	props := posthog.NewProperties().
		Set("message", message).
		Set("level", "info").
		Set("environment", phEnvironment).
		Set("service", phService)
	for k, v := range tags {
		props = props.Set(k, v)
	}
	_ = c.Enqueue(posthog.Capture{
		DistinctId: DistinctIDFromContext(ctx),
		Event:      "log_message",
		Properties: props,
	})
}

// FailureOptions configures CaptureFailure behavior.
type FailureOptions struct {
	Component string
	Operation string
	Tags      map[string]string
}

// CaptureFailure reports a failure consistently.
// Use this at boundary failure points (request, batch, task) rather than inside tight loops.
func CaptureFailure(ctx context.Context, err error, opts FailureOptions) {
	if err == nil {
		return
	}
	captureError(ctx, err, mergeFailureTags(opts))
}

func mergeFailureTags(opts FailureOptions) map[string]string {
	merged := make(map[string]string, len(opts.Tags)+2)
	for k, v := range opts.Tags {
		merged[k] = v
	}
	if opts.Component != "" {
		merged["component"] = opts.Component
	}
	if opts.Operation != "" {
		merged["operation"] = opts.Operation
	}
	return merged
}

// RecoverAndLog recovers from a panic, logs, reports to PostHog, and does not re-panic.
func RecoverAndLog(ctx context.Context, operation string) {
	if r := recover(); r != nil {
		err := fmt.Errorf("panic in %s: %v", operation, r)
		slog.ErrorContext(ctx, err.Error(), "operation", operation, "panic", true)
		CaptureFailure(ctx, err, FailureOptions{Operation: operation, Tags: map[string]string{"panic": "true"}})
	}
}

// NewHTTPMiddleware captures HTTP panics to PostHog before re-panicking (chi Recoverer still runs).
func NewHTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					var err error
					if e, ok := rec.(error); ok {
						err = e
					} else {
						err = fmt.Errorf("panic: %v", rec)
					}
					CaptureFailure(r.Context(), err, FailureOptions{Tags: map[string]string{"panic": "true", "path": r.URL.Path}})
					panic(rec)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

