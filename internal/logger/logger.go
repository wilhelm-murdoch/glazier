package logger

import (
	"context"
	"log"
	"log/slog"
	"os"
)

// Logger embeds the slog.Logger so that we can add support for
// additional logging levels.
type Logger struct {
	*slog.Logger
	Level slog.Level
}

// Critical provides support for critical log entries; exits
// immediately after submitting the record.
func (l *Logger) Critical(msg string, args ...any) {
	l.Log(context.Background(), LevelCritical, msg, args...)
	os.Exit(1)
}

// Trace provides support for trace log entries.
func (l *Logger) Trace(msg string, args ...any) {
	l.Log(context.Background(), LevelTrace, msg, args...)
}

// New returns a new logger set to the desired log level.
func New(level slog.Level) *Logger {
	return &Logger{
		Logger: slog.New(&Handler{
			Handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: level,
			}),
			l: log.New(os.Stdout, "", 0),
		}),
		Level: level,
	}
}
