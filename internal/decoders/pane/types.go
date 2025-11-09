package pane

import (
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
)

// Pane represents the configuration for a single tmux pane.
type Pane struct {
	*decoders.BaseDecoder
	Size     Size
	Commands []string
	Focus    bool
}

type Size struct {
	X, Y string
}

// Valid is a function that returns True if both X and Y values are not blank
func (s Size) Valid() bool {
	return s.X != "" && s.Y != ""
}

func New(spec cty.Value) *Pane {
	base := decoders.New(spec)

	return &Pane{
		BaseDecoder: base,
	}
}
