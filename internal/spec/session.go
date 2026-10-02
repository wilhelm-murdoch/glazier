package spec

import (
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

// Session returns the spec for the body of the session block; base is the directory of the profile.
// The parser finds the block itself, so variable blocks never reach it.
func Session(base string) hcldec.Spec {
	return &hcldec.ObjectSpec{
		"name": &hcldec.ValidateSpec{
			Wrapped: Name,
			Func:    diagnostics.SessionNameDiagnostic,
		},
		"starting_directory": startingDirectory(base),
		"hooks":              Hooks,
		"options":            Options,
		"windows":            window(base),
		"commands":           Commands,
		"envs": &hcldec.AttrSpec{
			Name: "envs",
			Type: cty.Map(cty.String),
		},
	}
}
