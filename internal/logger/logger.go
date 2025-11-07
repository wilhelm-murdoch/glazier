package logger

import (
	"context"
	"log"
	"log/slog"
	"os"
)

var (
	LevelDebugLabel    = "DBG"
	LevenInfoLabel     = "INF"
	LevelWarningLabel  = "WRN"
	LevelErrorLabel    = "ERR"
	LevelTraceLabel    = "TRC"
	LevelCriticalLabel = "CRT"

	LevelDebug    = slog.LevelDebug
	LevelInfo     = slog.LevelInfo
	LevelWarning  = slog.LevelWarn
	LevelError    = slog.LevelError
	LevelTrace    = slog.Level(-8)
	LevelCritical = slog.Level(12)
)

type Logger struct {
	*slog.Logger
	Level slog.Level
}

func (l *Logger) Critical(msg string, args ...any) {
	l.Log(context.Background(), LevelCritical, msg, args...)
	os.Exit(1)
}

func (l *Logger) Trace(msg string, args ...any) {
	l.Log(context.Background(), LevelTrace, msg, args...)
}

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
