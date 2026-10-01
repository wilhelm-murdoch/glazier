package diagnostics

import (
	"io"
	"os"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewWritesToStderr(t *testing.T) {
	stdout, stderr := captureStreams(t, func() {
		dm := New("profile.glaze", nil)
		dm.Append(&hcl.Diagnostic{Severity: hcl.DiagWarning, Summary: "Session name will be changed"})
		assert.NoError(t, dm.Write())
	})

	assert.Contains(t, stderr, "Session name will be changed")
	assert.Empty(t, stdout)

	// stderr is a pipe here, not a terminal, so the diagnostic has no colour codes.
	assert.NotContains(t, stderr, "\x1b[")
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
