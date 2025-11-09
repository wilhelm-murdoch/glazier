package window

import (
	"github.com/wilhelm-murdoch/go-collection"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/decoders/pane"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// Window represents the configuration for a single tmux window.
type Window struct {
	*decoders.BaseDecoder
	Panes  collection.Collection[*pane.Pane]
	Layout enums.Layout
	Focus  bool
}

func New(spec cty.Value) *Window {
	base := decoders.New(spec)

	return &Window{
		BaseDecoder: base,
	}
}
