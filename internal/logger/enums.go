package logger

import "log/slog"

const (
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
