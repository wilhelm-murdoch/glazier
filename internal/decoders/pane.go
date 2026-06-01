package decoders

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// Pane represents the configuration for a single tmux pane.
type Pane struct {
	*Base
	Size        Size
	Commands    []string
	Adjustments []Adjustment
	Focus       bool
}

type Size struct {
	X, Y string
}

// Adjustment represents a single directional pane resize step.
type Adjustment struct {
	Direction enums.Adjustment
	Amount    string
}

// Valid is a function that returns True if both X and Y values are not blank
func (s Size) Valid() bool {
	return s.X != "" && s.Y != ""
}

func NewPane(spec cty.Value) *Pane {
	base := NewBase(spec)

	return &Pane{
		Base: base,
	}
}

// Decode is responsible for decoding a cty.Value into a Pane struct.
func (p *Pane) Decode() hcl.Diagnostics {
	var diags hcl.Diagnostics

	focus := p.Spec.GetAttr("focus")
	if !focus.IsNull() {
		if err := gocty.FromCtyValue(focus, &p.Focus); err != nil {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid pane focus value",
				Detail:   err.Error(),
			})
		}
	}

	size := p.Spec.GetAttr("size")
	if !size.IsNull() {
		if x := size.GetAttr("x"); !x.IsNull() {
			p.Size.X = x.AsString()
		}
		if y := size.GetAttr("y"); !y.IsNull() {
			p.Size.Y = y.AsString()
		}
	}

	adjust := p.Spec.GetAttr("adjust")
	if !adjust.IsNull() && adjust.CanIterateElements() {
		adjustIterator := adjust.ElementIterator()

		for adjustIterator.Next() {
			_, spec := adjustIterator.Element()

			p.Adjustments = append(p.Adjustments, Adjustment{
				Direction: enums.AdjustmentFromString(spec.GetAttr("direction").AsString()),
				Amount:    spec.GetAttr("amount").AsString(),
			})
		}
	}

	commands := p.Spec.GetAttr("commands")
	if !commands.IsNull() && commands.CanIterateElements() {
		commandIterator := commands.ElementIterator()

		for commandIterator.Next() {
			_, command := commandIterator.Element()
			if command.Type().FriendlyName() == "string" {
				p.Commands = append(p.Commands, command.AsString())
			}
		}
	}

	return diags
}
