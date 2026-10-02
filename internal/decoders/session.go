package decoders

import (
	"fmt"
	"os"

	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/pkg/files"
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

// ResolveDirectories sets the absolute starting directory of the session, each window and each pane.
// A pane without one uses its window's, a window uses the session's, and the session uses the current directory.
func (s *Session) ResolveDirectories(baseDirectory string) error {
	// Only a session without a directory needs the current one, which can be gone, for example after a delete.
	if s.StartingDirectory == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("the session has no starting_directory, and glaze cannot read the current directory (%w); set starting_directory, or run glaze from a directory that exists", err)
		}

		s.StartingDirectory = pwd
	}

	var err error
	if s.StartingDirectory, err = files.ResolveDirectory(s.StartingDirectory, baseDirectory); err != nil {
		return err
	}

	for _, window := range s.Windows {
		if window.StartingDirectory, err = inherit(window.StartingDirectory, s.StartingDirectory, baseDirectory); err != nil {
			return err
		}

		for _, pane := range window.Panes {
			if pane.StartingDirectory, err = inherit(pane.StartingDirectory, window.StartingDirectory, baseDirectory); err != nil {
				return err
			}
		}
	}

	return nil
}

// inherit returns parent when dir is empty, or else dir resolved against baseDirectory.
func inherit(dir, parent, baseDirectory string) (string, error) {
	if dir == "" {
		return parent, nil
	}

	return files.ResolveDirectory(dir, baseDirectory)
}
