package diagnostics

import (
	"errors"
	"os"

	"github.com/hashicorp/hcl/v2"

	"github.com/wilhelm-murdoch/glazier/internal/term"
)

const diagnosticTextWriterWidth = 78

// ErrHasDiagnostics means that the diagnostics hold an error. Write has already printed them, so the caller only stops.
var ErrHasDiagnostics = errors.New("the glaze profile contains errors")

// DiagnosticsManager embeds the structure of hcl.Diagnostics and combines it
// with a DiagnosticsWriter to simplify use.
type DiagnosticsManager struct {
	hcl.Diagnostics
	Writer hcl.DiagnosticWriter
}

// Extend adds diags to the manager in place, unlike hcl.Diagnostics.Extend, which returns a new slice.
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

// New returns a manager that writes the diagnostics of the file at filePath to stderr, so stdout carries only command output.
func New(filePath string, file *hcl.File) *DiagnosticsManager {
	return &DiagnosticsManager{
		Diagnostics: hcl.Diagnostics{},
		Writer: hcl.NewDiagnosticTextWriter(
			os.Stderr,
			map[string]*hcl.File{filePath: file},
			diagnosticTextWriterWidth,
			term.ColorEnabled(os.Stderr),
		),
	}
}

// Report adds diags and writes every diagnostic, returning ErrHasDiagnostics when there is an error.
func (dm *DiagnosticsManager) Report(diags hcl.Diagnostics) error {
	dm.Extend(diags)
	return dm.Write()
}
