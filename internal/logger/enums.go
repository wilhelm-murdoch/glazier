package logger

import "log/slog"

const (
	LevelTraceLabel    = "TRC"
	LevelDebugLabel    = "DBG"
	LevelInfoLabel     = "INF"
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

var FriendlyToInternal = map[string]slog.Level{
	"trace":    LevelTrace,
	"debug":    LevelDebug,
	"info":     LevelInfo,
	"warning":  LevelWarning,
	"error":    LevelError,
	"critical": LevelCritical,
}
