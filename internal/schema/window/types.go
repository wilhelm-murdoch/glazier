package window

import (
	"github.com/wilhelm-murdoch/glazier/internal/schema"
	"github.com/wilhelm-murdoch/glazier/internal/schema/pane"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
	"github.com/wilhelm-murdoch/go-collection"
	"github.com/zclconf/go-cty/cty"
)

// Window represents the configuration for a single tmux window.
type Window struct {
	*schema.BaseSchema
	Panes  collection.Collection[*pane.Pane]
	Layout enums.Layout
	Focus  bool
}

func New(spec cty.Value) *Window {
	base := schema.New(spec)

	return &Window{
		BaseSchema: base,
	}
}
