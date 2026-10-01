package logger

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWritesToStderr(t *testing.T) {
	stdout, stderr := captureStreams(t, func() {
		New(LevelInfo).Info("provisioning")
	})

	assert.Contains(t, stderr, "provisioning")
	assert.Empty(t, stdout)
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
