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

	Envs = &hcldec.AttrSpec{
		Name: "envs",
		Type: cty.Map(cty.String),
	}

	Name = &hcldec.AttrSpec{
		Name: "name",
		Type: cty.String,
	}

	StartingDirectory = &hcldec.ValidateSpec{
		Wrapped: &hcldec.AttrSpec{
			Name: "starting_directory",
			Type: cty.String,
		},
		Func: func(value cty.Value) hcl.Diagnostics {
			return diagnostics.DirectoryDiagnostic(
				"starting directory",
				value,
			)
		},
	}
)
