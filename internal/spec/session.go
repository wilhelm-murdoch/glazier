package spec

import (
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"
)

var Session = &hcldec.BlockSpec{
	TypeName: "session",
	Required: true,
	Nested: &hcldec.ObjectSpec{
		"name":               Name,
		"starting_directory": StartingDirectory,
		"envs":               Envs,
		"hooks":              Hooks,
		"windows":            Window,
		"commands": &hcldec.AttrSpec{
			Name: "commands",
			Type: cty.List(cty.String),
		},
	},
}
