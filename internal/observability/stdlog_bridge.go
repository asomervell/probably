package observability

import (
	"log"
	"log/slog"
	"strings"
)

type stdLogToSlogWriter struct{}

func (w stdLogToSlogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		lower := strings.ToLower(msg)
		switch {
		case strings.Contains(lower, "panic"), strings.Contains(lower, "fatal"), strings.Contains(lower, "error"), strings.Contains(lower, "failed"):
			slog.Error(msg)
		case strings.Contains(lower, "warn"), strings.Contains(lower, "warning"):
			slog.Warn(msg)
		default:
			slog.Info(msg)
		}
	}
	return len(p), nil
}

// RedirectStdLogToSlog routes standard-library log output through slog.
// This ensures legacy log.Printf lines are exported via OTEL when slog is wired.
func RedirectStdLogToSlog() {
	log.SetFlags(0)
	log.SetOutput(stdLogToSlogWriter{})
}
