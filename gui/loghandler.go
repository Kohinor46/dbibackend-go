package gui

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// lineHandler formats records as "15:04:05 INFO message key=value".
type lineHandler struct {
	mu    sync.Mutex
	w     io.Writer
	level slog.Leveler
	attrs []slog.Attr
}

func (h *lineHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level.Level() }

func (h *lineHandler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %-5s %s", r.Time.Format("15:04:05"), r.Level, r.Message)
	write := func(a slog.Attr) bool {
		fmt.Fprintf(&sb, " %s=%v", a.Key, a.Value)
		return true
	}
	for _, a := range h.attrs {
		write(a)
	}
	r.Attrs(write)
	sb.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, sb.String())
	return err
}

func (h *lineHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &lineHandler{w: h.w, level: h.level, attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...)}
}

func (h *lineHandler) WithGroup(string) slog.Handler { return h }
