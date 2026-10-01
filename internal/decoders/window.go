package decoders

import (
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// Window is a decoded window block.
type Window struct {
	*Base
	Panes []*Pane

	// Layout is the named preset, or enums.LayoutUnknown when LayoutRaw holds a raw tmux layout string.
	Layout    enums.Layout
	LayoutRaw string
	Focus     bool
}

// NewWindow decodes a window block, with its panes. The layout defaults to tiled.
func NewWindow(spec cty.Value) *Window {
	window := &Window{
		Base:   NewBase(spec),
		Layout: enums.LayoutTiled,
		Focus:  isTrue(spec.GetAttr("focus")),
	}

	if layout := spec.GetAttr("layout"); !layout.IsNull() {
		window.Layout = enums.LayoutFromString(layout.AsString())
		if window.Layout == enums.LayoutUnknown {
			window.LayoutRaw = layout.AsString()
		}
	}

	for _, pane := range elements(spec.GetAttr("panes")) {
		window.Panes = append(window.Panes, NewPane(pane))
	}

	return window
}

// LayoutValue returns the layout to pass to tmux: the raw layout string, or else the name of the preset.
func (w *Window) LayoutValue() string {
	if w.Layout == enums.LayoutUnknown {
		return w.LayoutRaw
	}

	return w.Layout.String()
}
