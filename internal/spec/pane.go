package spec

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

var Pane = &hcldec.BlockListSpec{
	TypeName: "pane",
	MinItems: 1,
	Nested: &hcldec.ObjectSpec{
		"name":               nameSpec("pane"),
		"starting_directory": StartingDirectory,
		"hooks":              Hooks,
		"options":            Options,
		"focus":              Focus,
		"commands":           Commands,
		"size": &hcldec.ValidateSpec{
			Wrapped: &hcldec.BlockSpec{
				TypeName: "size",
				Nested: hcldec.ObjectSpec{
					"x": sizeSpec("x", false, diagnostics.SizeDiagnostic),
					"y": sizeSpec("y", false, diagnostics.SizeDiagnostic),
				},
			},
			Func: func(value cty.Value) hcl.Diagnostics {
				if value.IsNull() || !value.GetAttr("x").IsNull() || !value.GetAttr("y").IsNull() {
					return nil
				}

				return diagnostics.Invalid("size", "A size block must have a valid `x` and or `y` attribute.")
			},
		},
		"adjust": &hcldec.BlockListSpec{
			TypeName: "adjust",
			MaxItems: 4,
			Nested: hcldec.ObjectSpec{
				"direction": &hcldec.ValidateSpec{
					Wrapped: &hcldec.AttrSpec{
						Name:     "direction",
						Type:     cty.String,
						Required: true,
					},
					Func: func(value cty.Value) hcl.Diagnostics {
						return diagnostics.ContainsDiagnostic("direction", value, enums.AdjustmentList)
					},
				},
				"amount": sizeSpec("amount", true, diagnostics.AmountDiagnostic),
			},
		},
	},
}
