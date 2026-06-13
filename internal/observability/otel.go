package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/asomervell/probably/internal/config"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracerProvider *sdktrace.TracerProvider
	loggerProvider *sdklog.LoggerProvider
	tracer         trace.Tracer
)

// Tracer returns the application tracer (noop until InitOTEL succeeds).
func Tracer() trace.Tracer {
	if tracer != nil {
		return tracer
	}
	return trace.NewNoopTracerProvider().Tracer("probably")
}

// InitOTEL configures OTLP export of logs and AI traces to PostHog.
func InitOTEL(ctx context.Context, cfg *config.Config, serviceName string) (shutdown func(context.Context) error, err error) {
	if cfg.PostHogProjectAPIKey == "" {
		slog.Info("OTEL export disabled; PostHog project key not set")
		tracer = trace.NewNoopTracerProvider().Tracer("probably")
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(serviceName),
			attribute.String("deployment.environment", firstNonEmpty(cfg.PostHogEnvironment, cfg.Environment)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	host := cfg.PostHogOTELHost
	if host == "" {
		host = "us.i.posthog.com"
	}
	authHeader := map[string]string{
		"Authorization": "Bearer " + cfg.PostHogProjectAPIKey,
	}

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(host),
		otlptracehttp.WithURLPath("/i/v0/ai/otel"),
		otlptracehttp.WithHeaders(authHeader),
	)
	if err != nil {
		return nil, fmt.Errorf("otel trace exporter: %w", err)
	}

	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter,
			// PostHog AI OTEL endpoint rejects batches > 100 AI spans.
			// MaxQueueSize caps the in-flight queue so a shutdown flush
			// never sends more than one batch (≤50 spans → well under 100).
			sdktrace.WithMaxExportBatchSize(50),
			sdktrace.WithMaxQueueSize(75),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)
	tracer = tracerProvider.Tracer("github.com/asomervell/probably")

	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(host),
		otlploghttp.WithURLPath("/i/v1/logs"),
		otlploghttp.WithHeaders(authHeader),
	)
	if err != nil {
		_ = tracerProvider.Shutdown(ctx)
		tracerProvider = nil
		return nil, fmt.Errorf("otel log exporter: %w", err)
	}

	loggerProvider = sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	global.SetLoggerProvider(loggerProvider)

	// Surface OTLP export failures (wrong region, bad token, network) in server logs.
	// Rate-limited to once per minute to avoid log spam when the endpoint returns persistent errors (e.g. 403).
	var (
		otelErrMu      sync.Mutex
		otelErrLastLog time.Time
	)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		if err == nil {
			return
		}
		otelErrMu.Lock()
		shouldLog := time.Since(otelErrLastLog) >= time.Minute
		if shouldLog {
			otelErrLastLog = time.Now()
		}
		otelErrMu.Unlock()
		if shouldLog {
			slog.Error("OTEL export error", "error", err)
			CaptureFailure(context.Background(), err, FailureOptions{
				Component: "otel",
				Operation: "export",
				Tags: map[string]string{
					"otel.host": host,
				},
			})
		}
	}))

	slog.Info("OTEL initialized OTLP export to PostHog", "host", host)

	return func(shutdownCtx context.Context) error {
		var errs []error
		if loggerProvider != nil {
			if e := loggerProvider.Shutdown(shutdownCtx); e != nil {
				errs = append(errs, e)
			}
			loggerProvider = nil
		}
		if tracerProvider != nil {
			if e := tracerProvider.Shutdown(shutdownCtx); e != nil {
				errs = append(errs, e)
			}
			tracerProvider = nil
		}
		tracer = nil
		if len(errs) > 0 {
			return errs[0]
		}
		return nil
	}, nil
}

// NewOTELSLogHandler returns an slog.Handler that ships logs to PostHog via OTEL (nil if disabled).
func NewOTELSLogHandler(cfg *config.Config) *otelslog.Handler {
	if cfg.PostHogProjectAPIKey == "" {
		return nil
	}
	return otelslog.NewHandler("probably", otelslog.WithLoggerProvider(global.GetLoggerProvider()))
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
