package window

import (
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/wilhelm-murdoch/go-collection"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"

	"github.com/wilhelm-murdoch/glazier/internal/schema"
	"github.com/wilhelm-murdoch/glazier/internal/schema/pane"
	"github.com/wilhelm-murdoch/glazier/internal/tmux/enums"
)

const DefaultGlazeWindowName = "default"

// Window represents the configuration for a single tmux window.
type Window struct {
	schema.Base
	Name              schema.Name
	StartingDirectory schema.Directory
	Options           schema.Options
	Panes             collection.Collection[*pane.Pane]
	Layout            enums.Layout
	Focus             schema.Focus
}

// Decode is responsible for decoding a cty.Value into a Window struct.
func (w *Window) Decode(window cty.Value) hcl.Diagnostics {
	var diags hcl.Diagnostics

	name := window.GetAttr("name")
	if !name.IsNull() {
		w.Name = schema.Name(name.AsString())
	} else {
		w.Name = DefaultGlazeWindowName
	}

	layout := window.GetAttr("layout")
	if !layout.IsNull() {
		w.Layout = enums.LayoutFromString(layout.AsString())
	} else {
		w.Layout = enums.LayoutTiled
	}

	focus := window.GetAttr("focus")
	if !focus.IsNull() {
		gocty.FromCtyValue(focus, &w.Focus)
	}

	startingDirectory := window.GetAttr("starting_directory")
	if !startingDirectory.IsNull() {
		w.StartingDirectory = schema.Directory(startingDirectory.AsString())
	} else {
		if pwd, err := os.Getwd(); err == nil {
			w.StartingDirectory = schema.Directory(pwd)
		}
	}

	envs := window.GetAttr("envs")
	if !envs.IsNull() {
		w.Envs = w.DecodeEnvs(envs)
	}

	panes := window.GetAttr("panes")
	if panes.CanIterateElements() {
		paneIterator := panes.ElementIterator()

		for paneIterator.Next() {
			_, element := paneIterator.Element()

			pane := new(pane.Pane)
			if diags = pane.Decode(element); diags.HasErrors() {
				diags = diags.Extend(diags)
				continue
			}

			w.Panes.Push(pane)
		}
	}

	return diags
}
