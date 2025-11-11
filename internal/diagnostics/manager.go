package diagnostics

import (
	"os"

	"github.com/hashicorp/hcl/v2"
)

// DiagnosticsManager embeds the structure of hcl.Diagnostics and combines it
// with a DiagnosticsWriter to simplify use.
type DiagnosticsManager struct {
	hcl.Diagnostics
	Writer hcl.DiagnosticWriter
}

// Write is responsible for writing the diagnostics to the DiagnosticWriter.
func (dm *DiagnosticsManager) Write() error {
	return dm.Writer.WriteDiagnostics(dm.Diagnostics)
}

// NewDiagnosticsManager is responsible for creating a new DiagnosticsManager instance.
func New(filePath string, file *hcl.File) *DiagnosticsManager {
	return &DiagnosticsManager{
		Diagnostics: hcl.Diagnostics{},
		Writer: hcl.NewDiagnosticTextWriter(
			os.Stdout,
			map[string]*hcl.File{filePath: file},
			78,
			true,
		),
	}
}
