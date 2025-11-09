package session

import (
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/wilhelm-murdoch/go-collection"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/schema"
	"github.com/wilhelm-murdoch/glazier/internal/schema/window"
)

const DefaultGlazeSesssionName = "default"

type Session struct {
	schema.BaseSchema
	Windows  collection.Collection[*window.Window]
	Commands []string
}

func New() (*Session, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	return &Session{}, diags
}

// Decode is responsible for decoding a cty.Value into a Session struct.
func (s *Session) Decode(session cty.Value) hcl.Diagnostics {
	var diags hcl.Diagnostics

	name := session.GetAttr("name")
	if !name.IsNull() {
		s.Name = name.AsString()
	} else {
		s.Name = DefaultGlazeSesssionName
	}

	startingDirectory := session.GetAttr("starting_directory")
	if !startingDirectory.IsNull() {
		s.StartingDirectory = startingDirectory.AsString()
	} else {
		if pwd, err := os.Getwd(); err == nil {
			s.StartingDirectory = pwd
		}
	}

	envs := session.GetAttr("envs")
	if !envs.IsNull() {
		s.Envs = s.DecodeEnvs(envs)
	}

	commands := session.GetAttr("commands")
	if !commands.IsNull() {
		s.Commands = s.DecodeCommands(commands)
	}

	windows := session.GetAttr("windows")
	if windows.CanIterateElements() {
		windowIterator := windows.ElementIterator()

		for windowIterator.Next() {
			_, element := windowIterator.Element()

			window := new(window.Window)
			if diags = window.Decode(element); diags.HasErrors() {
				diags = diags.Extend(diags)
				continue
			}

			s.Windows.Push(window)
		}
	}

	return diags
}
