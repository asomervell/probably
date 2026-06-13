package observability

import (
	"context"
	"log/slog"
)

// NewMultiSlogHandler forwards each record to every non-nil handler (e.g. stdout + OTLP).
func NewMultiSlogHandler(handlers ...slog.Handler) slog.Handler {
	var hs []slog.Handler
	for _, h := range handlers {
		if h != nil {
			hs = append(hs, h)
		}
	}
	switch len(hs) {
	case 0:
		return slog.DiscardHandler
	case 1:
		return hs[0]
	default:
		return &multiSlogHandler{handlers: hs}
	}
}

type multiSlogHandler struct {
	handlers []slog.Handler
}

func (m *multiSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *multiSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		out[i] = h.WithAttrs(attrs)
	}
	return &multiSlogHandler{handlers: out}
}

func (m *multiSlogHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		out[i] = h.WithGroup(name)
	}
	return &multiSlogHandler{handlers: out}
}
