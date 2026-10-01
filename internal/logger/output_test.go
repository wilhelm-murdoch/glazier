package logger

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWritesToStderr(t *testing.T) {
	stdout, stderr := captureStreams(t, func() {
		New(LevelInfo).Info("provisioning")
	})

	assert.Contains(t, stderr, "provisioning")
	assert.Empty(t, stdout)

	// stderr is a pipe here, not a terminal, so the log line has no colour codes.
	assert.NotContains(t, stderr, "\x1b[")
}

func TestHandlerColor(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		var buf bytes.Buffer
		h := newTestHandler(&buf, LevelInfo)
		h.color = enabled

		rec := slog.NewRecord(time.Now(), slog.LevelWarn, "careful", 0)
		assert.NoError(t, h.Handle(context.Background(), rec))

		assert.Equal(t, enabled, strings.Contains(buf.String(), "\x1b["), "colour enabled = %v", enabled)
		assert.Contains(t, buf.String(), LevelWarningLabel)
	}
}

// captureStreams returns what fn writes to os.Stdout and os.Stderr.
func captureStreams(t *testing.T, fn func()) (string, string) {
	t.Helper()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create a pipe: %v", err)
	}

	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create a pipe: %v", err)
	}

	previousOut, previousErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = previousOut, previousErr }()

	fn()
	_ = outW.Close()
	_ = errW.Close()

	out, _ := io.ReadAll(outR)
	errOut, _ := io.ReadAll(errR)

	return string(out), string(errOut)
}
