package window

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty/gocty"

	"github.com/wilhelm-murdoch/glazier/internal/schema/pane"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

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
		gocty.FromCtyValue(focus, &w.Focus)
	}

	panes := w.Spec.GetAttr("panes")
	if panes.CanIterateElements() {
		paneIterator := panes.ElementIterator()

		for paneIterator.Next() {
			_, spec := paneIterator.Element()

			pane := pane.New(spec)
			if diags = pane.Decode(); diags.HasErrors() {
				diags = diags.Extend(diags)
				continue
			}

			w.Panes.Push(pane)
		}
	}

	return diags
}
