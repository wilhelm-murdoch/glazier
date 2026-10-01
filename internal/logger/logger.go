package logger

import (
	"log"
	"log/slog"
	"os"
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
			l: log.New(os.Stderr, "", 0),
		}),
		Level: level,
	}
}
