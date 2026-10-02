package spec

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

var (
	// Hooks maps a tmux hook name to a command. The name must be one that tmux knows, and no command can be null.
	Hooks = &hcldec.ValidateSpec{
		Wrapped: &hcldec.AttrSpec{
			Name: "hooks",
			Type: cty.Map(cty.String),
		},
		Func: diagnostics.HooksDiagnostic,
	}

	Options = noNulls("options", cty.Map(cty.String))

	Name = &hcldec.AttrSpec{
		Name: "name",
		Type: cty.String,
	}

	// SessionName is the session `name`, which `up` and `down` decode the same way. It warns about characters that tmux rewrites.
	SessionName = &hcldec.ValidateSpec{
		Wrapped: Name,
		Func:    diagnostics.SessionNameDiagnostic,
	}

	// Focus makes a window or a pane the active one.
	Focus = &hcldec.AttrSpec{
		Name: "focus",
		Type: cty.Bool,
	}

	// Commands run in a pane, or in the active pane for a session.
	Commands = noNulls("commands", cty.List(cty.String))

	// Envs is the environment of the session.
	Envs = noNulls("envs", cty.Map(cty.String))
)

// startingDirectory returns the starting_directory attribute. A relative path is relative to baseDirectory, the directory of the profile.
func startingDirectory(baseDirectory string) hcldec.Spec {
	return &hcldec.ValidateSpec{
		Wrapped: &hcldec.AttrSpec{
			Name: "starting_directory",
			Type: cty.String,
		},
		Func: func(value cty.Value) hcl.Diagnostics {
			return diagnostics.DirectoryDiagnostic("starting directory", value, baseDirectory)
		},
	}
}

// nameSpec returns the name attribute of a window or a pane, which warns about characters that tmux rewrites.
func nameSpec(kind string) hcldec.Spec {
	return &hcldec.ValidateSpec{
		Wrapped: Name,
		Func: func(value cty.Value) hcl.Diagnostics {
			return diagnostics.NameDiagnostic(kind, value)
		},
	}
}

// noNulls returns a list or a map attribute that rejects a null element, because glaze cannot pass a null to tmux.
func noNulls(name string, ty cty.Type) hcldec.Spec {
	return &hcldec.ValidateSpec{
		Wrapped: &hcldec.AttrSpec{
			Name: name,
			Type: ty,
		},
		Func: func(value cty.Value) hcl.Diagnostics {
			return diagnostics.NoNullsDiagnostic(name, value)
		},
	}
}

// sizeSpec returns a string attribute for a size or an amount, which validate checks. A required one must not be null.
func sizeSpec(name string, required bool, validate func(field string, value cty.Value) hcl.Diagnostics) hcldec.Spec {
	return &hcldec.ValidateSpec{
		Wrapped: &hcldec.AttrSpec{
			Name:     name,
			Type:     cty.String,
			Required: required,
		},
		Func: func(value cty.Value) hcl.Diagnostics {
			if required {
				if diags := diagnostics.RequiredDiagnostic(name, value); diags.HasErrors() {
					return diags
				}
			}

			return validate(name, value)
		},
	}
}
