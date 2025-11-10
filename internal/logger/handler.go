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
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()

	switch r.Level {
	case slog.LevelDebug:
		level = color.MagentaString(LevelDebugLabel)
	case slog.LevelInfo:
		level = color.BlueString(LevelInfoLabel)
	case slog.LevelWarn:
		level = color.YellowString(LevelWarningLabel)
	case slog.LevelError:
		level = color.RedString(LevelErrorLabel)
	case LevelTrace:
		level = color.BlackString(LevelTraceLabel)
	case LevelCritical:
		red := color.New(color.FgRed).Add(color.Bold)
		level = red.Sprint(LevelCriticalLabel)
	}

	var fields []string
	r.Attrs(func(a slog.Attr) bool {
		// For the purpose of this project, we assume all values are strings
		fields = append(fields, color.BlackString(a.Key+"=")+a.Value.Resolve().String())

		return true
	})

	h.l.Println(
		color.WhiteString(
			r.Time.Format(time.DateTime),
		),
		level,
		r.Message,
		strings.Join(fields, " "),
	)

	return nil
}
