package pane

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/internal/tmux/enums"
)

var Spec = &hcldec.BlockListSpec{
	TypeName: "pane",
	MinItems: 1,
	Nested: &hcldec.ValidateSpec{
		Wrapped: &hcldec.ObjectSpec{
			"name": &hcldec.AttrSpec{
				Name: "name",
				Type: cty.String,
			},
			"envs": &hcldec.AttrSpec{
				Name: "hooks",
				Type: cty.Map(cty.String),
			},
			"hooks": &hcldec.AttrSpec{
				Name: "hooks",
				Type: cty.Map(cty.String),
			},
			"focus": &hcldec.AttrSpec{
				Name: "focus",
				Type: cty.Bool,
			},
			"size": &hcldec.ValidateSpec{
				Wrapped: &hcldec.BlockSpec{
					TypeName: "size",
					Nested: hcldec.ObjectSpec{
						"x": &hcldec.ValidateSpec{
							Wrapped: &hcldec.AttrSpec{
								Name: "x",
								Type: cty.String,
							},
							Func: func(value cty.Value) hcl.Diagnostics {
								return diagnostics.WrongSizeDiagnostic(
									"x",
									value,
								)
							},
						},
						"y": &hcldec.ValidateSpec{
							Wrapped: &hcldec.AttrSpec{
								Name: "y",
								Type: cty.String,
							},
							Func: func(value cty.Value) hcl.Diagnostics {
								return diagnostics.WrongSizeDiagnostic(
									"y",
									value,
								)
							},
						},
					},
				},
				Func: func(value cty.Value) hcl.Diagnostics {
					var out hcl.Diagnostics
					if !value.IsNull() {
						return hcl.Diagnostics{{
							Severity: hcl.DiagError,
							Summary:  "Invalid size specified",
							Detail:   "A size block must have a valid `x` and or `y` attribute.",
						}}
					}
					return out
				},
			},
			"adjust": &hcldec.BlockListSpec{
				TypeName: "adjust",
				MinItems: 0,
				MaxItems: 4,
				Nested: hcldec.ObjectSpec{
					"direction": &hcldec.ValidateSpec{
						Wrapped: &hcldec.AttrSpec{
							Name:     "direction",
							Type:     cty.String,
							Required: true,
						},
						Func: func(value cty.Value) hcl.Diagnostics {
							return diagnostics.ContainsDiagnostic(
								"direction",
								value,
								enums.AdjustmentList,
							)
						},
					},
					"amount": &hcldec.ValidateSpec{
						Wrapped: &hcldec.AttrSpec{
							Name:     "amount",
							Type:     cty.String,
							Required: true,
						},
						Func: func(value cty.Value) hcl.Diagnostics {
							return diagnostics.WrongSizeDiagnostic(
								"amount",
								value,
							)
						},
					},
				},
			},
			"starting_directory": &hcldec.ValidateSpec{
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
			},
			"commands": &hcldec.AttrSpec{
				Name: "commands",
				Type: cty.List(cty.String),
			},
		},
		Func: func(value cty.Value) hcl.Diagnostics {
			var diags hcl.Diagnostics

			// Placeholder for future schema validations ...

			return diags
		},
	},
}
