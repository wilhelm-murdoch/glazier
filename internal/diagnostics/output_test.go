package diagnostics

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

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

func TestWriteSortsBySource(t *testing.T) {
	at := func(file string, offset int) *hcl.Range {
		return &hcl.Range{Filename: file, Start: hcl.Pos{Byte: offset, Line: 1, Column: 1}, End: hcl.Pos{Byte: offset + 1, Line: 1, Column: 2}}
	}

	dm := New("p.glaze", nil)
	dm.Append(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "b.hcl", Subject: at("b.hcl", 0)})
	dm.Append(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "profile late", Subject: at("p.glaze", 50)})
	dm.Append(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "no place"})
	dm.Append(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "a.hcl", Subject: at("a.hcl", 9)})
	dm.Append(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "profile early", Subject: at("p.glaze", 2)})

	captureStreams(t, func() { assert.ErrorIs(t, dm.Write(), ErrHasDiagnostics) })

	var order []string
	for _, diag := range dm.Diagnostics {
		order = append(order, diag.Summary)
	}

	assert.Equal(t, []string{"no place", "profile early", "profile late", "a.hcl", "b.hcl"}, order)
}

func TestWriteShowsTheLineOfAnotherFile(t *testing.T) {
	vars := filepath.Join(t.TempDir(), "vars.hcl")
	assert.NoError(t, os.WriteFile(vars, []byte("n = \"abc\"\n"), 0o600))

	_, stderr := captureStreams(t, func() {
		dm := New("p.glaze", nil)
		dm.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid variable value",
			Subject:  &hcl.Range{Filename: vars, Start: hcl.Pos{Line: 1, Column: 1}, End: hcl.Pos{Line: 1, Column: 10, Byte: 9}},
		})

		assert.Error(t, dm.Write())
	})

	assert.Contains(t, stderr, `1: n = "abc"`)
	assert.NotContains(t, stderr, "source code not available")
}

func TestWriteDoesNotReadAPipe(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "vars.hcl")
	assert.NoError(t, syscall.Mkfifo(fifo, 0o600))

	// A read of a FIFO with no writer blocks, so the test fails after a timeout when Write reads it.
	done := make(chan struct{})
	go func() {
		defer close(done)
		captureStreams(t, func() {
			dm := New("p.glaze", nil)
			dm.Append(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Invalid variable value", Subject: &hcl.Range{Filename: fifo}})
			assert.Error(t, dm.Write())
		})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Write blocked on the FIFO")
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
