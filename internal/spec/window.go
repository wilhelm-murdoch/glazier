package spec

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// window returns the spec for the window blocks of a session; base is the directory of the profile.
func window(base string) hcldec.Spec {
	return &hcldec.BlockListSpec{
		TypeName: "window",
		MinItems: 1,
		Nested: &hcldec.ObjectSpec{
			"name":               nameSpec("window"),
			"starting_directory": startingDirectory(base),
			"hooks":              Hooks,
			"options":            Options,
			"panes":              pane(base),
			"focus":              Focus,
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
}
