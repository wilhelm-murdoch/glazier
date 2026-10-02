package spec

import (
	"github.com/hashicorp/hcl/v2/hcldec"
)

// Session returns the spec for the body of the session block; baseDirectory is the directory
// of the profile. The parser finds the block itself, so variable blocks never reach it.
func Session(baseDirectory string) hcldec.Spec {
	return &hcldec.ObjectSpec{
		"name":               SessionName,
		"starting_directory": startingDirectory(baseDirectory),
		"hooks":              Hooks,
		"options":            Options,
		"windows":            window(baseDirectory),
		"commands":           Commands,
		"envs":               Envs,
	}
}
