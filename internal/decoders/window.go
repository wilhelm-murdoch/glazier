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
	Panes  collection.Collection[*Pane]
	Layout enums.Layout
	Focus  bool
}

func NewWindow(spec cty.Value) *Window {
	base := NewBase(spec)

	return &Window{
		Base: base,
	}
}

// Decode is responsible for decoding a cty.Value into a Window struct.
func (w *Window) Decode() hcl.Diagnostics {
	var diags hcl.Diagnostics

	layout := w.Spec.GetAttr("layout")
	if !layout.IsNull() {
		w.Layout = enums.LayoutFromString(layout.AsString())
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
