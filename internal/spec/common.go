package spec

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

var (
	Hooks = &hcldec.AttrSpec{
		Name: "hooks",
		Type: cty.Map(cty.String),
	}

	Options = &hcldec.AttrSpec{
		Name: "options",
		Type: cty.Map(cty.String),
	}

	Name = &hcldec.AttrSpec{
		Name: "name",
		Type: cty.String,
	}

	// Focus makes a window or a pane the active one.
	Focus = &hcldec.AttrSpec{
		Name: "focus",
		Type: cty.Bool,
	}

	// Commands run in a pane, or in the active pane for a session.
	Commands = &hcldec.AttrSpec{
		Name: "commands",
		Type: cty.List(cty.String),
	}

	StartingDirectory = &hcldec.ValidateSpec{
		Wrapped: &hcldec.AttrSpec{
			Name: "starting_directory",
			Type: cty.String,
		},
		Func: func(value cty.Value) hcl.Diagnostics {
			return diagnostics.DirectoryDiagnostic("starting directory", value)
		},
	}
)

// nameSpec returns the name attribute of a window or a pane, which warns about characters that tmux rewrites.
func nameSpec(kind string) hcldec.Spec {
	return &hcldec.ValidateSpec{
		Wrapped: Name,
		Func: func(value cty.Value) hcl.Diagnostics {
			return diagnostics.NameDiagnostic(kind, value)
		},
	}
}

// sizeSpec returns a size attribute in cells or as a percentage.
func sizeSpec(name string, required bool) hcldec.Spec {
	return &hcldec.ValidateSpec{
		Wrapped: &hcldec.AttrSpec{
			Name:     name,
			Type:     cty.String,
			Required: required,
		},
		Func: func(value cty.Value) hcl.Diagnostics {
			return diagnostics.WrongSizeDiagnostic(name, value)
		},
	}
}
