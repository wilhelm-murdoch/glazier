package logger

import "log/slog"

const (
	LevelTraceLabel    = "TRC"
	LevelDebugLabel    = "DBG"
	LevenInfoLabel     = "INF"
	LevelWarningLabel  = "WRN"
	LevelErrorLabel    = "ERR"
	LevelCriticalLabel = "CRT"

	LevelTrace    = slog.Level(-8)
	LevelDebug    = slog.LevelDebug
	LevelInfo     = slog.LevelInfo
	LevelWarning  = slog.LevelWarn
	LevelError    = slog.LevelError
	LevelCritical = slog.Level(12)
)
