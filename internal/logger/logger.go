package logger

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
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
		level = color.MagentaString("DBG")
	case slog.LevelInfo:
		level = color.BlueString("INF")
	case slog.LevelWarn:
		level = color.YellowString("WRN")
	case slog.LevelError:
		level = color.RedString("ERR")
	}

	var fields []string
	r.Attrs(func(a slog.Attr) bool {
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

func NewHandler(out io.Writer, opts slog.HandlerOptions) *Handler {
	return &Handler{
		Handler: slog.NewTextHandler(out, &opts),
		l:       log.New(out, "", 0),
	}
}

func New(level slog.Level) *slog.Logger {
	return slog.New(&Handler{
		Handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		}),
		l: log.New(os.Stdout, "", 0),
	})
}
