package logger

import "log/slog"

// The log levels of glaze and their three-letter labels.
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

// FriendlyToInternal maps the words of --log-level to slog levels.
var FriendlyToInternal = map[string]slog.Level{
	"trace":    LevelTrace,
	"debug":    LevelDebug,
	"info":     LevelInfo,
	"warning":  LevelWarning,
	"error":    LevelError,
	"critical": LevelCritical,
}
