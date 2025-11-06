package format

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/urfave/cli/v2"

	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/schema"
)

type Action struct {
	actions.BaseAction
}

// NewAction is responsible for creating a new Action instance for the format command.
func NewAction(ctx *cli.Context, logger *slog.Logger) (*Action, error) {
	base, err := actions.NewBaseAction(ctx, logger)
	if err != nil {
		return nil, err
	}

	return &Action{
		BaseAction: *base,
	}, nil
}

// Run is an action that will reformat the given glaze definition file to match
// a canonical format and style, ensuring consistency.
func (a *Action) Run() error {
	formatted := string(hclwrite.Format(a.Parser.File.Bytes))

	if a.Context.Bool("validate") {
		if valid := a.isGlazeDefintionValid(); !valid {
			return a.DiagnosticsManager.Write()
		}
	}

	if a.Context.Bool("stdout") {
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
func (a *Action) isGlazeDefintionValid() bool {
	variables, err := parser.CollectVariables(a.Context.StringSlice("var"))
	if err != nil {
		a.DiagnosticsManager.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("could not parse specified variables: %s", err),
			Detail:   err.Error(),
		})

		return false
	}

	_, decodeDiags := a.Parser.Decode(
		schema.PrimaryGlazeSpec,
		parser.BuildEvalContext(variables),
	)

	if decodeDiags.HasErrors() {
		a.DiagnosticsManager.Extend(decodeDiags)
		return false
	}

	return true
}
