package logger

import (
	"context"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/fatih/color"
)

type Handler struct {
	slog.Handler
	l *log.Logger

	// color is true when the handler writes to a terminal that accepts colour.
	color bool
}

// paint colours s with attrs only when the handler may write colour.
func (h *Handler) paint(s string, attrs ...color.Attribute) string {
	c := color.New(attrs...)
	if h.color {
		c.EnableColor()
	} else {
		c.DisableColor()
	}

	return c.Sprint(s)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()

	switch r.Level {
	case slog.LevelDebug:
		level = h.paint(LevelDebugLabel, color.FgMagenta)
	case slog.LevelInfo:
		level = h.paint(LevelInfoLabel, color.FgBlue)
	case slog.LevelWarn:
		level = h.paint(LevelWarningLabel, color.FgYellow)
	case slog.LevelError:
		level = h.paint(LevelErrorLabel, color.FgRed)
	case LevelTrace:
		level = h.paint(LevelTraceLabel, color.FgBlack)
	case LevelCritical:
		level = h.paint(LevelCriticalLabel, color.FgRed, color.Bold)
	}

	var fields []string
	r.Attrs(func(a slog.Attr) bool {
		// For the purpose of this project, we assume all values are strings
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
