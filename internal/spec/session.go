package spec

import (
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"
)

// Session is the hcldec specification for the *body* of a session block. The
// parser locates the single session block itself and decodes its body against
// this spec, which is why there is no enclosing BlockSpec here: top-level
// sibling blocks (notably `variable` declarations) are handled before this
// point and never reach the session decode.
var Session = &hcldec.ObjectSpec{
	"name":               Name,
	"starting_directory": StartingDirectory,
	"hooks":              Hooks,
	"options":            Options,
	"windows":            Window,
	"commands": &hcldec.AttrSpec{
		Name: "commands",
		Type: cty.List(cty.String),
	},
	"envs": &hcldec.AttrSpec{
		Name: "envs",
		Type: cty.Map(cty.String),
	},
}
