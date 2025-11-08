package pane

import (
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"

	"github.com/wilhelm-murdoch/glazier/internal/schema"
)

const DefaultGlazePaneName = "default"

// Pane represents the configuration for a single tmux pane.
type Pane struct {
	schema.Base
	StartingDirectory schema.Directory
	Size              schema.Size
	Options           schema.Options
	Commands          schema.Commands
	Focus             schema.Focus
}

// Decode is responsible for decoding a cty.Value into a Pane struct.
func (p *Pane) Decode(pane cty.Value) hcl.Diagnostics {
	var diags hcl.Diagnostics

	if !pane.GetAttr("name").IsNull() {
		p.Name = schema.Name(pane.GetAttr("name").AsString())
	} else {
		p.Name = DefaultGlazePaneName
	}

	focus := pane.GetAttr("focus")
	if !focus.IsNull() {
		gocty.FromCtyValue(focus, &p.Focus)
	}

	startingDirectory := pane.GetAttr("starting_directory")
	if !startingDirectory.IsNull() {
		p.StartingDirectory = schema.Directory(startingDirectory.AsString())
	} else {
		if pwd, err := os.Getwd(); err == nil {
			p.StartingDirectory = schema.Directory(pwd)
		}
	}

	size := pane.GetAttr("size")
	if !size.IsNull() {
		p.Size.X = size.GetAttr("x").AsString()
		p.Size.Y = size.GetAttr("y").AsString()
	}

	envs := pane.GetAttr("envs")
	if !envs.IsNull() {
		p.Envs = p.DecodeEnvs(envs)
	}

	hooks := pane.GetAttr("hooks")
	if !hooks.IsNull() {
		p.Hooks = make(schema.Hooks)
		for name, hook := range hooks.AsValueMap() {
			p.Hooks[schema.Name(name)] = schema.Value(hook.AsString())
		}
	}

	commands := pane.GetAttr("commands")
	if commands.CanIterateElements() {
		commandIterator := commands.ElementIterator()

		for commandIterator.Next() {
			_, command := commandIterator.Element()
			if command.Type().FriendlyName() == "string" {
				p.Commands = append(p.Commands, schema.Command(command.AsString()))
			}
		}
	}

	return diags
}
