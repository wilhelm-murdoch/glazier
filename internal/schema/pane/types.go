package pane

import (
	"github.com/wilhelm-murdoch/glazier/internal/schema"
	"github.com/zclconf/go-cty/cty"
)

// Pane represents the configuration for a single tmux pane.
type Pane struct {
	*schema.BaseSchema
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
	base := schema.New(spec)

	return &Pane{
		BaseSchema: base,
	}
}
