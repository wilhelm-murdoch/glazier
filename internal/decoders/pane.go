package decoders

import (
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// Pane is a decoded pane block.
type Pane struct {
	*Base
	Size        Size
	Commands    []string
	Adjustments []Adjustment
	Focus       bool
}

// Size is the width and the height of a pane, in cells or as a percentage.
type Size struct {
	X, Y string
}

// Adjustment is one directional resize of a pane.
type Adjustment struct {
	Direction enums.Adjustment
	Amount    string
}

// IsSet reports whether the width or the height is set.
func (s Size) IsSet() bool {
	return s.X != "" || s.Y != ""
}

// NewPane decodes a pane block.
func NewPane(spec cty.Value) *Pane {
	pane := &Pane{
		Base:     NewBase(spec),
		Commands: stringList(spec.GetAttr("commands")),
		Focus:    isTrue(spec.GetAttr("focus")),
	}

	if size := spec.GetAttr("size"); !size.IsNull() {
		if x := size.GetAttr("x"); !x.IsNull() {
			pane.Size.X = x.AsString()
		}

		if y := size.GetAttr("y"); !y.IsNull() {
			pane.Size.Y = y.AsString()
		}
	}

	for _, adjust := range elements(spec.GetAttr("adjust")) {
		pane.Adjustments = append(pane.Adjustments, Adjustment{
			Direction: enums.AdjustmentFromString(adjust.GetAttr("direction").AsString()),
			Amount:    adjust.GetAttr("amount").AsString(),
		})
	}

	return pane
}
