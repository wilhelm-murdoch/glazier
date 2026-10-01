package diagnostics

import (
	"errors"
	"os"

	"github.com/hashicorp/hcl/v2"
)

const diagnosticTextWriterWidth = 78

// ErrHasDiagnostics is returned by Write when the accumulated set contains
// error-level diagnostics. The detailed diagnostics have already been rendered
// to the writer, so callers can halt execution without re-printing them.
var ErrHasDiagnostics = errors.New("the glaze profile contains errors")

// DiagnosticsManager embeds the structure of hcl.Diagnostics and combines it
// with a DiagnosticsWriter to simplify use.
type DiagnosticsManager struct {
	hcl.Diagnostics
	Writer hcl.DiagnosticWriter
}

// Extend appends the given diagnostics to the accumulated set. It shadows the
// embedded hcl.Diagnostics.Extend (which returns a new slice the caller must
// reassign) so that accumulation mutates the manager in place.
func (dm *DiagnosticsManager) Extend(diags hcl.Diagnostics) {
	dm.Diagnostics = dm.Diagnostics.Extend(diags)
}

// Append appends a single diagnostic to the accumulated set. It shadows the
// embedded hcl.Diagnostics.Append for the same in-place reason as Extend.
func (dm *DiagnosticsManager) Append(diag *hcl.Diagnostic) {
	dm.Diagnostics = dm.Diagnostics.Append(diag)
}

// Write writes every diagnostic and returns ErrHasDiagnostics when there is an error, so the caller stops.
func (dm *DiagnosticsManager) Write() error {
	if writeErr := dm.Writer.WriteDiagnostics(dm.Diagnostics); writeErr != nil {
		return writeErr
	}

	if dm.HasErrors() {
		return ErrHasDiagnostics
	}

	return nil
}

// NewDiagnosticsManager is responsible for creating a new DiagnosticsManager instance.
// Diagnostics go to stderr, so stdout carries only command output.
func New(filePath string, file *hcl.File) *DiagnosticsManager {
	return &DiagnosticsManager{
		Diagnostics: hcl.Diagnostics{},
		Writer: hcl.NewDiagnosticTextWriter(
			os.Stderr,
			map[string]*hcl.File{filePath: file},
			diagnosticTextWriterWidth,
			true,
		),
	}
}

// Report adds diags and writes every diagnostic, returning ErrHasDiagnostics when there is an error.
func (dm *DiagnosticsManager) Report(diags hcl.Diagnostics) error {
	dm.Extend(diags)
	return dm.Write()
}
