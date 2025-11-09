package pane

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty/gocty"
)

// Decode is responsible for decoding a cty.Value into a Pane struct.
func (p *Pane) Decode() hcl.Diagnostics {
	var diags hcl.Diagnostics

	focus := p.Spec.GetAttr("focus")
	if !focus.IsNull() {
		gocty.FromCtyValue(focus, &p.Focus)
	}

	size := p.Spec.GetAttr("size")
	if !size.IsNull() {
		p.Size.X = size.GetAttr("x").AsString()
		p.Size.Y = size.GetAttr("y").AsString()
	}

	commands := p.Spec.GetAttr("commands")
	if commands.CanIterateElements() {
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
