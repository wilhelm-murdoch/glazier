package logger

import (
	"log"
	"log/slog"
	"os"

	"github.com/wilhelm-murdoch/glazier/internal/term"
)

// Logger is a slog.Logger that remembers its level.
type Logger struct {
	*slog.Logger
	Level slog.Level
}

// New returns a new logger set to the desired log level. It writes to stderr, so stdout carries only command output.
func New(level slog.Level) *Logger {
	return &Logger{
		Logger: slog.New(&Handler{
			Handler: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			}),
			l:     log.New(os.Stderr, "", 0),
			color: term.ColorEnabled(os.Stderr),
		}),
		Level: level,
	}
}
