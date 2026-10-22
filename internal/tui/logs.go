package tui

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"unicode"
)

type Logs struct {
	mu    sync.Mutex
	lines []string
}
type logHandler struct {
	logs  *Logs
	attrs []slog.Attr
	group string
}

func (l *Logs) Logger() *slog.Logger { return slog.New(&logHandler{logs: l}) }
func (l *Logs) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}
func (h *logHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *logHandler) Handle(_ context.Context, r slog.Record) error {
	line := r.Time.Format("15:04:05") + " " + r.Level.String() + " " + r.Message
	attrs := append([]slog.Attr(nil), h.attrs...)
	r.Attrs(func(a slog.Attr) bool { attrs = append(attrs, a); return true })
	for _, a := range attrs {
		line += " " + h.group + a.Key + "=" + a.Value.String()
	}
	line = safe(line)
	if len(line) > 1000 {
		line = line[:1000]
	}
	h.logs.mu.Lock()
	defer h.logs.mu.Unlock()
	h.logs.lines = append(h.logs.lines, line)
	if len(h.logs.lines) > 8 {
		h.logs.lines = h.logs.lines[len(h.logs.lines)-8:]
	}
	return nil
}
func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &logHandler{logs: h.logs, attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...), group: h.group}
}
func (h *logHandler) WithGroup(group string) slog.Handler {
	return &logHandler{logs: h.logs, attrs: append([]slog.Attr(nil), h.attrs...), group: h.group + group + "."}
}
func safe(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' {
			return ' '
		}
		return r
	}, text)
}
