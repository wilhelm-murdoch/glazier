package diagnostics

import (
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

type DiagnosticsManager struct {
	Diagnostics      hcl.Diagnostics
	DiagnosticWriter hcl.DiagnosticWriter
}

// Write is responsible for writing the diagnostics to the DiagnosticWriter.
func (dm *DiagnosticsManager) Write() error {
	return dm.DiagnosticWriter.WriteDiagnostics(dm.Diagnostics)
}

// Extend is responsible for extending the existing diagnostics with new ones.
func (dm *DiagnosticsManager) Extend(diags hcl.Diagnostics) hcl.Diagnostics {
	dm.Diagnostics = dm.Diagnostics.Extend(diags)
	return dm.Diagnostics
}

// Append is responsible for appending a single diagnostic to the existing ones.
func (dm *DiagnosticsManager) Append(diag *hcl.Diagnostic) hcl.Diagnostics {
	dm.Diagnostics = dm.Diagnostics.Append(diag)
	return dm.Diagnostics
}

// HasErrors is responsible for checking if there are any errors in the diagnostics.
func (dm *DiagnosticsManager) HasErrors() bool {
	return dm.Diagnostics.HasErrors()
}

// NewDiagnosticsManager is responsible for creating a new DiagnosticsManager instance.
func New(filePath string) *DiagnosticsManager {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCLFile(filePath)

	diagsManager := &DiagnosticsManager{
		DiagnosticWriter: hcl.NewDiagnosticTextWriter(
			os.Stdout,
			map[string]*hcl.File{filePath: file},
			78,
			true,
		),
	}

	if diags.HasErrors() {
		diagsManager.Extend(diags)
	}

	return diagsManager
}
