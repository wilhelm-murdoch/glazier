package decoders

import (
	"github.com/zclconf/go-cty/cty"
)

// Session is the decoded session block of a profile.
type Session struct {
	*Base
	Envs     map[string]string
	Windows  []*Window
	Commands []string
}

// NewSession decodes a session block, with its windows and panes.
func NewSession(spec cty.Value) *Session {
	session := &Session{
		Base:     NewBase(spec),
		Envs:     stringMap(spec.GetAttr("envs")),
		Commands: stringList(spec.GetAttr("commands")),
	}

	for _, window := range elements(spec.GetAttr("windows")) {
		session.Windows = append(session.Windows, NewWindow(window))
	}

	return session
}
