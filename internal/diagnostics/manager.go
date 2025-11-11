package diagnostics

import (
	"os"

	"github.com/hashicorp/hcl/v2"
)

type DiagnosticsManager struct {
	hcl.Diagnostics
	DiagnosticWriter hcl.DiagnosticWriter
}

// Write is responsible for writing the diagnostics to the DiagnosticWriter.
func (dm *DiagnosticsManager) Write() error {
	return dm.DiagnosticWriter.WriteDiagnostics(dm.Diagnostics)
}

// NewDiagnosticsManager is responsible for creating a new DiagnosticsManager instance.
func New(filePath string, file *hcl.File) *DiagnosticsManager {
	return &DiagnosticsManager{
		Diagnostics: hcl.Diagnostics{},
		DiagnosticWriter: hcl.NewDiagnosticTextWriter(
			os.Stdout,
			map[string]*hcl.File{filePath: file},
			78,
			true,
		),
	}
}
