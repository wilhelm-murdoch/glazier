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
	schema.BaseSchema
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

// Decode is responsible for decoding a cty.Value into a Pane struct.
func (p *Pane) Decode(pane cty.Value) hcl.Diagnostics {
	var diags hcl.Diagnostics

	name := pane.GetAttr("name")
	if !name.IsNull() {
		p.Name = name.AsString()
	} else {
		p.Name = DefaultGlazePaneName
	}

	focus := pane.GetAttr("focus")
	if !focus.IsNull() {
		gocty.FromCtyValue(focus, &p.Focus)
	}

	startingDirectory := pane.GetAttr("starting_directory")
	if !startingDirectory.IsNull() {
		p.StartingDirectory = startingDirectory.AsString()
	} else {
		if pwd, err := os.Getwd(); err == nil {
			p.StartingDirectory = pwd
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
		p.Hooks = make(map[string]string)
		for name, hook := range hooks.AsValueMap() {
			p.Hooks[name] = hook.AsString()
		}
	}

	commands := pane.GetAttr("commands")
	if commands.CanIterateElements() {
		commandIterator := commands.ElementIterator()

		for commandIterator.Next() {
			_, command := commandIterator.Element()
			if command.Type().FriendlyName() == "string" {
				p.Commands = append(p.Commands, command.AsString())
			}
		}
	}

	return diags
}
