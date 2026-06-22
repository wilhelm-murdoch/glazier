package decoders

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Session struct {
	*Base
	Envs     map[string]string
	Windows  []*Window
	Commands []string
}

func NewSession(spec cty.Value) *Session {
	base := NewBase(spec)

	session := &Session{
		Base: base,
	}

	envs := spec.GetAttr("envs")
	if !envs.IsNull() {
		session.Envs = make(map[string]string, len(envs.AsValueMap()))
		for name, value := range envs.AsValueMap() {
			session.Envs[name] = value.AsString()
		}
	}

	return session
}

// Decode is responsible for decoding a cty.Value into a Session struct.
func (s *Session) Decode() hcl.Diagnostics {
	var allDiags hcl.Diagnostics

	commands := s.Spec.GetAttr("commands")
	if !commands.IsNull() && commands.CanIterateElements() {
		commandIterator := commands.ElementIterator()

		for commandIterator.Next() {
			_, command := commandIterator.Element()
			if command.Type().FriendlyName() == "string" {
				s.Commands = append(s.Commands, command.AsString())
			}
		}
	}

	windows := s.Spec.GetAttr("windows")
	if windows.CanIterateElements() {
		windowIterator := windows.ElementIterator()

		for windowIterator.Next() {
			_, spec := windowIterator.Element()

			window := NewWindow(spec)
			windowDiags := window.Decode()
			if windowDiags.HasErrors() {
				allDiags = allDiags.Extend(windowDiags)
			}

			s.Windows = append(s.Windows, window)
		}
	}

	return allDiags
}
