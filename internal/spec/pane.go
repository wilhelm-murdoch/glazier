package spec

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// pane returns the spec for the pane blocks of a window; baseDirectory is the directory of the profile.
func pane(baseDirectory string) hcldec.Spec {
	return &hcldec.BlockListSpec{
		TypeName: "pane",
		MinItems: 1,
		Nested: &hcldec.ObjectSpec{
			"name":               nameSpec("pane"),
			"starting_directory": startingDirectory(baseDirectory),
			"hooks":              Hooks,
			"options":            Options,
			"focus":              Focus,
			"commands":           Commands,
			"size": &hcldec.ValidateSpec{
				Wrapped: &hcldec.BlockSpec{
					TypeName: "size",
					Nested: &hcldec.ObjectSpec{
						"x": stringAttr("x", false, diagnostics.SizeDiagnostic),
						"y": stringAttr("y", false, diagnostics.SizeDiagnostic),
					},
				},
				Func: func(value cty.Value) hcl.Diagnostics {
					if value.IsNull() || !value.GetAttr("x").IsNull() || !value.GetAttr("y").IsNull() {
						return nil
					}

					return diagnostics.Invalid("size", "A size block must set `x`, `y` or both.")
				},
			},
			"adjust": &hcldec.BlockListSpec{
				TypeName: "adjust",
				MaxItems: 4,
				Nested: &hcldec.ObjectSpec{
					"direction": stringAttr("direction", true, func(field string, value cty.Value) hcl.Diagnostics {
						return diagnostics.ContainsDiagnostic(field, value, enums.AdjustmentList)
					}),
					"amount": stringAttr("amount", true, diagnostics.AmountDiagnostic),
				},
			},
		},
	}
}
