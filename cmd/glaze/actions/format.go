package actions

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/spec"
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
func (a *ActionFormat) Run() error {
	formatted := string(hclwrite.Format(a.Parser.File.Bytes))

	if a.Command.Bool("validate") {
		if validationDiags := a.isGlazeDefinitionValid(); validationDiags != nil {
			a.DiagnosticsManager.Extend(validationDiags)
			return a.DiagnosticsManager.Write()
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

// isGlazeDefinitionValid checks if the given glaze definition file and any
// variable flags yield a valid result when run through the parser. Validation
// is strict (requireAll): a declared variable with no default and no --var
// value is reported, the same as it would be on `up`.
func (a *ActionFormat) isGlazeDefinitionValid() hcl.Diagnostics {
	ctx, ctxDiags := a.Parser.VariableContext(a.Command.StringSlice("var"), a.Command.String("var-file"), true)
	if ctxDiags.HasErrors() {
		return ctxDiags
	}

	if _, decodeDiags := a.Parser.Decode(spec.Session, ctx); decodeDiags.HasErrors() {
		return decodeDiags
	}

	return nil
}
