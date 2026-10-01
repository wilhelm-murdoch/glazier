package actions

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/urfave/cli/v3"
)

// ActionFormat is a struct that represents a Glazier "action".
type ActionFormat struct {
	ActionBase
}

// NewFormat is responsible for creating a new ActionFormat struct value pre-populated
// with fields that are common across all other action structs.
func NewFormat(cmd *cli.Command, logLevel string) (*ActionFormat, error) {
	base, err := NewActionBase(cmd, logLevel)
	if err != nil {
		return nil, err
	}

	return &ActionFormat{
		ActionBase: *base,
	}, nil
}

// Run is a method that reformats the given glaze definition file to match a canonical
// format and style, ensuring consistency.
func (a *ActionFormat) Run(_ context.Context) error {
	formatted := string(hclwrite.Format(a.Parser.File.Bytes))

	if a.Command.Bool("validate") {
		_, validationDiags := a.decodeProfile()
		if validationDiags.HasErrors() {
			return a.DiagnosticsManager.Report(validationDiags)
		}

		// Warnings do not stop the format. Show them and continue.
		if len(validationDiags) > 0 {
			if err := a.DiagnosticsManager.Writer.WriteDiagnostics(validationDiags); err != nil {
				return err
			}
		}
	}

	if a.Command.Bool("stdout") {
		fmt.Print(formatted)
		return nil
	}

	// Profiles are sharable config meant to be committed; 0644 is intended.
	if err := os.WriteFile(a.ProfilePath, []byte(formatted), 0o644); err != nil { //nolint:gosec // G306
		a.DiagnosticsManager.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Failed to write file",
			Detail:   err.Error(),
		})
	}

	if a.DiagnosticsManager.HasErrors() {
		return a.DiagnosticsManager.Write()
	}

	return nil
}
