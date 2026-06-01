package logger

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

// newTestHandler builds a Handler writing to the returned buffer so emitted
// records can be inspected without touching stdout.
func newTestHandler(buf *bytes.Buffer, level slog.Level) *Handler {
	return &Handler{
		Handler: slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level}),
		l:       log.New(buf, "", 0),
	}
}

func TestHandlerHandleLevels(t *testing.T) {
	color.NoColor = true

	cases := []struct {
		name  string
		level slog.Level
		label string
	}{
		{"debug", slog.LevelDebug, LevelDebugLabel},
		{"info", slog.LevelInfo, LevelInfoLabel},
		{"warning", slog.LevelWarn, LevelWarningLabel},
		{"error", slog.LevelError, LevelErrorLabel},
		{"trace", LevelTrace, LevelTraceLabel},
		{"critical", LevelCritical, LevelCriticalLabel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := newTestHandler(&buf, LevelTrace)

			rec := slog.NewRecord(time.Now(), tc.level, "hello world", 0)
			rec.AddAttrs(slog.String("key", "value"))

			assert.NoError(t, h.Handle(context.Background(), rec))

			out := buf.String()
			assert.Contains(t, out, tc.label)
			assert.Contains(t, out, "hello world")
			assert.Contains(t, out, "key=")
			assert.Contains(t, out, "value")
		})
	}
}

func TestHandlerHandleNoAttrs(t *testing.T) {
	color.NoColor = true

	var buf bytes.Buffer
	h := newTestHandler(&buf, LevelInfo)

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "no attrs", 0)
	assert.NoError(t, h.Handle(context.Background(), rec))
	assert.Contains(t, buf.String(), "no attrs")
}

func TestNew(t *testing.T) {
	l := New(LevelInfo)
	assert.NotNil(t, l)
	assert.Equal(t, LevelInfo, l.Level)
	assert.NotNil(t, l.Logger)
}

func TestLoggerTrace(t *testing.T) {
	// Trace below the configured level should be filtered out by the handler.
	l := New(LevelInfo)
	assert.NotPanics(t, func() {
		l.Trace("trace message", "key", "value")
	})
}

func TestFriendlyToInternal(t *testing.T) {
	assert.Equal(t, LevelTrace, FriendlyToInternal["trace"])
	assert.Equal(t, LevelCritical, FriendlyToInternal["critical"])
	_, ok := FriendlyToInternal["nonsense"]
	assert.False(t, ok)
}
