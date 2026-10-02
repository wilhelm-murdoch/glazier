package spec

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// window returns the spec for the window blocks of a session; baseDirectory is the directory
// of the profile.
func window(baseDirectory string) hcldec.Spec {
	return &hcldec.BlockListSpec{
		TypeName: "window",
		MinItems: 1,
		// A raw layout must describe as many panes as the window declares, so the check needs the whole window.
		Nested: &hcldec.ValidateSpec{
			Func: diagnostics.LayoutCellsDiagnostic,
			Wrapped: &hcldec.ObjectSpec{
				"name":               nameSpec("window"),
				"starting_directory": startingDirectory(baseDirectory),
				"hooks":              Hooks,
				"options":            Options,
				"panes":              pane(baseDirectory),
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
		},
	}
}
