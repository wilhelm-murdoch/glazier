package tmux

import (
	"fmt"
)

type SessionId int

// String returns the tmux id of the session, for example $3.
func (id SessionId) String() string {
	return fmt.Sprintf("$%d", int(id))
}

// Session is a tmux session.
type Session struct {
	Client            Client
	Name              string
	StartingDirectory string
	Id                SessionId
}

// Target returns the id that tmux commands use to address the session.
func (s Session) Target() string {
	return s.Id.String()
}

// NewWindow creates a window in the session and returns it. An empty startingDirectory uses the default of tmux.
func (s *Session) NewWindow(windowName, startingDirectory string) (*Window, error) {
	args := []string{
		"neww", "-d",
		"-t", s.Target(),
		"-n", escapeFormat(SanitizeName(windowName)),
		"-F", formatActiveWindows, "-P",
	}

	if startingDirectory != "" {
		args = append(args, "-c", escapeFormat(startingDirectory))
	}

	output, err := s.Client.output(args...)
	if err != nil {
		return nil, err
	}

	return s.Client.NewWindowFromLine(output, s)
}

// Kill kills the session.
func (s Session) Kill() error {
	return s.Client.run("kill-session", "-t", s.Target())
}

// SetEnv sets an environment variable on the session, which only processes that start later inherit.
// "--" ends the flags, so a key or a value that starts with "-" stays an operand.
func (s Session) SetEnv(key, value string) error {
	return s.Client.run("setenv", "-t", s.Target(), "--", key, value)
}

// SetHook sets a session hook.
func (s Session) SetHook(hook, command string) error {
	return s.Client.setScoped("set-hook", "", s.Target(), hook, command)
}

// SetOption sets a session option.
func (s Session) SetOption(option, value string) error {
	return s.Client.setScoped("set-option", "", s.Target(), option, value)
}

// Host returns the host name that tmux reports, which is also the title that tmux gives a new pane.
func (s Session) Host() (string, error) {
	return s.Client.output("display-message", "-p", "-t", s.Target(), "#{host}")
}

// ActivePane returns the id of the active pane of the session.
func (s Session) ActivePane() (string, error) {
	pane, err := s.Client.output("display-message", "-p", "-t", s.Target(), "#{pane_id}")
	if err != nil {
		return "", fmt.Errorf("could not find the active pane of session `%s`: %w", s.Name, err)
	}

	return pane, nil
}
