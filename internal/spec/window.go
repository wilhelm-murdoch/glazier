package spec

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

var Window = &hcldec.BlockListSpec{
	TypeName: "window",
	MinItems: 1,
	Nested: &hcldec.ObjectSpec{
		"name":               Name,
		"starting_directory": StartingDirectory,
		"envs":               Envs,
		"hooks":              Hooks,
		"options":            Options,
		"panes":              Pane,
		"focus": &hcldec.AttrSpec{
			Name: "focus",
			Type: cty.Bool,
		},
		"layout": &hcldec.ValidateSpec{
			Wrapped: &hcldec.AttrSpec{
				Name: "layout",
				Type: cty.String,
			},
			Func: func(value cty.Value) hcl.Diagnostics {
				return diagnostics.LayoutDiagnostic("layout", value, enums.LayoutList)
			},
		},
	},
}
