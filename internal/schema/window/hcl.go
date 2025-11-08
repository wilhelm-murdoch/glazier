package window

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/internal/schema/pane"
	"github.com/wilhelm-murdoch/glazier/internal/tmux/enums"
)

var Spec = &hcldec.BlockListSpec{
	TypeName: "window",
	MinItems: 1,
	Nested: &hcldec.ObjectSpec{
		"name": &hcldec.AttrSpec{
			Name: "name",
			Type: cty.String,
		},
		"envs": &hcldec.AttrSpec{
			Name: "envs",
			Type: cty.Map(cty.String),
		},
		"hooks": &hcldec.AttrSpec{
			Name: "hooks",
			Type: cty.Map(cty.String),
		},
		"focus": &hcldec.AttrSpec{
			Name: "focus",
			Type: cty.Bool,
		},
		"starting_directory": &hcldec.ValidateSpec{
			Wrapped: &hcldec.AttrSpec{
				Name: "starting_directory",
				Type: cty.String,
			},
			Func: func(value cty.Value) hcl.Diagnostics {
				return diagnostics.DirectoryDiagnostic("starting directory", value)
			},
		},
		"layout": &hcldec.ValidateSpec{
			Wrapped: &hcldec.AttrSpec{
				Name: "layout",
				Type: cty.String,
			},
			Func: func(value cty.Value) hcl.Diagnostics {
				return diagnostics.ContainsDiagnostic("layout", value, enums.LayoutList)
			},
		},
		"panes": pane.Spec,
	},
}
