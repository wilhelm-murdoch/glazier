package decoders

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/wilhelm-murdoch/go-collection"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// Window represents the configuration for a single tmux window.
type Window struct {
	*Base
	Panes collection.Collection[*Pane]
	// Layout is the named preset. When it resolves to enums.LayoutUnknown the
	// declared value was a raw tmux layout coordinate string, preserved in
	// LayoutRaw (use LayoutValue to get the value to hand to tmux).
	Layout    enums.Layout
	LayoutRaw string
	Focus     bool
}

func NewWindow(spec cty.Value) *Window {
	base := NewBase(spec)

	return &Window{
		Base: base,
	}
}

// LayoutValue returns the layout string to pass to tmux: the raw coordinate
// string when the declared layout is not a named preset, otherwise the preset.
func (w *Window) LayoutValue() string {
	if w.Layout == enums.LayoutUnknown {
		return w.LayoutRaw
	}

	return w.Layout.String()
}

// Decode is responsible for decoding a cty.Value into a Window struct.
func (w *Window) Decode() hcl.Diagnostics {
	var diags hcl.Diagnostics

	layout := w.Spec.GetAttr("layout")
	if !layout.IsNull() {
		raw := layout.AsString()
		w.Layout = enums.LayoutFromString(raw)
		if w.Layout == enums.LayoutUnknown {
			w.LayoutRaw = raw
		}
	} else {
		w.Layout = enums.LayoutTiled
	}

	focus := w.Spec.GetAttr("focus")
	if !focus.IsNull() {
		if err := gocty.FromCtyValue(focus, &w.Focus); err != nil {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid window focus value",
				Detail:   err.Error(),
			})
		}
	}

	panes := w.Spec.GetAttr("panes")
	if panes.CanIterateElements() {
		paneIterator := panes.ElementIterator()

		for paneIterator.Next() {
			_, spec := paneIterator.Element()

			pane := NewPane(spec)
			if diags = pane.Decode(); diags.HasErrors() {
				diags = diags.Extend(diags)
				continue
			}

			w.Panes.Push(pane)
		}
	}

	return diags
}
