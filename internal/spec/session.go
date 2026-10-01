package spec

import (
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

// Session is the spec for the body of the session block. The parser finds the block itself, so variable blocks never reach it.
var Session = &hcldec.ObjectSpec{
	"name": &hcldec.ValidateSpec{
		Wrapped: Name,
		Func:    diagnostics.SessionNameDiagnostic,
	},
	"starting_directory": StartingDirectory,
	"hooks":              Hooks,
	"options":            Options,
	"windows":            Window,
	"commands":           Commands,
	"envs": &hcldec.AttrSpec{
		Name: "envs",
		Type: cty.Map(cty.String),
	},
}
