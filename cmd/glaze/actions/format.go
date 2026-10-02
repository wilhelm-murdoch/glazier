package actions

import (
	"context"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/pkg/files"
)

// ActionFormat formats a profile, and validates it with --validate.
type ActionFormat struct {
	ActionBase
}

// NewFormat returns the format action, with the profile parsed.
// It refuses an in-place format of a pipe or a device before it reads it, because a read can block and a write cannot replace it.
func NewFormat(cmd *cli.Command, logLevel string) (*ActionFormat, error) {
	if !cmd.Bool("stdout") {
		if path, err := files.ResolveProfilePath(cmd.String("profile-path")); err == nil && !files.IsRegular(path) {
			return nil, fmt.Errorf("could not format `%s` in place: %w; use --stdout", path, files.ErrNotRegular)
		}
	}

	base, err := NewActionBase(cmd, logLevel)
	if err != nil {
		return nil, err
	}

	return &ActionFormat{
		ActionBase: *base,
	}, nil
}

// Run rewrites the profile in the canonical HCL format, or prints it with --stdout.
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

	// An unchanged file keeps its modification time, so editors and file watchers see no change.
	if formatted == string(a.Parser.File.Bytes) {
		a.Logger.Info("the profile is already formatted", "path", a.ProfilePath)
		return nil
	}

	// Profiles are sharable config meant to be committed; 0644 is intended.
	if err := files.WriteFile(a.ProfilePath, []byte(formatted), 0o644); err != nil {
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
