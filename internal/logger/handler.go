package logger

import (
	"context"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/wilhelm-murdoch/glazier/internal/term"
)

// Handler writes one line per record, with colour when the output is a terminal.
type Handler struct {
	slog.Handler
	l *log.Logger

	// color is true when the handler writes to a terminal that accepts colour.
	color bool
}

// paint colours s with attrs only when the handler may write colour.
func (h *Handler) paint(s string, attrs ...color.Attribute) string {
	return term.Paint(h.color, s, attrs...)
}

// Handle formats r as a time, a level label, the message and key=value pairs.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()

	switch r.Level {
	case LevelDebug:
		level = h.paint(LevelDebugLabel, color.FgMagenta)
	case LevelInfo:
		level = h.paint(LevelInfoLabel, color.FgBlue)
	case LevelWarning:
		level = h.paint(LevelWarningLabel, color.FgYellow)
	case LevelError:
		level = h.paint(LevelErrorLabel, color.FgRed)
	case LevelTrace:
		level = h.paint(LevelTraceLabel, color.FgBlack)
	case LevelCritical:
		level = h.paint(LevelCriticalLabel, color.FgRed, color.Bold)
	}

	var fields []string
	r.Attrs(func(a slog.Attr) bool {
		// Resolve expands a LogValuer before String formats the value.
		fields = append(fields, h.paint(a.Key+"=", color.FgBlack)+a.Value.Resolve().String())

		return true
	})

	h.l.Println(
		h.paint(r.Time.Format(time.DateTime), color.FgWhite),
		level,
		r.Message,
		strings.Join(fields, " "),
	)

	return nil
}
