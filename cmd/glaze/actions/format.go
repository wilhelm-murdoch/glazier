package actions

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

type ActionFormat struct {
	ActionBase
}

// NewAction is responsible for creating a new Action instance for the format command.
func NewFormat(cmd *cli.Command, logger *logger.Logger) (*ActionFormat, error) {
	base, err := NewActionBase(cmd, logger)
	if err != nil {
		return nil, err
	}

	return &ActionFormat{
		ActionBase: *base,
	}, nil
}

// Run is an action that will reformat the given glaze definition file to match
// a canonical format and style, ensuring consistency.
func (a *ActionFormat) Run() error {
	formatted := string(hclwrite.Format(a.Parser.File.Bytes))

	if a.Command.Bool("validate") {
		if valid := a.isGlazeDefintionValid(); !valid {
			return a.DiagnosticsManager.Write()
		}
	}

	if a.Command.Bool("stdout") {
		fmt.Print(formatted)
		return nil
	}

	if err := os.WriteFile(a.ProfilePath, []byte(formatted), 0o644); err != nil {
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

// isGlazeDefintionValid checks if the given glaze definition file and any variable
// flags yield a valid result when run through the schema.Parser.
func (a *ActionFormat) isGlazeDefintionValid() bool {
	variables, err := parser.CollectVariables(a.Command.StringSlice("var"))
	if err != nil {
		a.DiagnosticsManager.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("could not parse specified variables: %s", err),
			Detail:   err.Error(),
		})

		return false
	}

	_, decodeDiags := a.Parser.Decode(
		spec.Session,
		parser.BuildEvalContext(variables),
	)

	if decodeDiags.HasErrors() {
		a.DiagnosticsManager.Extend(decodeDiags)
		return false
	}

	return true
}
