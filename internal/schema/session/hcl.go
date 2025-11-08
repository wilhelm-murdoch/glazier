package session

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/internal/schema/window"
)

var Spec = &hcldec.BlockListSpec{
	TypeName: "session",
	MinItems: 1,
	MaxItems: 1,
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
		"commands": &hcldec.AttrSpec{
			Name: "commands",
			Type: cty.List(cty.String),
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
		"windows": window.Spec,
	},
}
