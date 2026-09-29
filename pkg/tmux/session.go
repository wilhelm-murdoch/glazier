package tmux

import (
	"fmt"
	"log/slog"
)

type SessionId int

// String is responsible for returning the string representation of the SessionId.
func (id SessionId) String() string {
	return fmt.Sprintf("$%d", int(id))
}

// Session represents a tmux session.
type Session struct {
	Client            Client
	Name              string
	StartingDirectory string
	Id                SessionId
	logger            *slog.Logger
}

// Target returns the target session by its string representation of the SessionId.
func (s Session) Target() string {
	return s.Id.String()
}

// NewWindow creates a new window in the current session and returns it.
func (s *Session) NewWindow(windowName, startingDirectory string) (*Window, error) {
	var window *Window

	args := []string{
		"neww",
		"-d",
		"-t", s.Target(),
		"-n", fmt.Sprint(windowName),
		"-F", formatActiveWindows,
		"-P",
	}

	if startingDirectory != "" {
		args = append(args, "-c", startingDirectory)
	}

	cmd := newCommand(s.Client, args...)

	s.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return window, err
	}

	window, err = s.Client.NewWindowFromLine(output, s)
	if err != nil {
		return window, err
	}

	return window, nil
}

// Kill closes the current session.
func (s Session) Kill() error {
	cmd := newCommand(s.Client, "kill-session", "-t", s.Target())

	s.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SetEnv sets an environment variable on the session. tmux scopes environment
// variables to sessions, so this is the natural level at which to apply them.
func (s Session) SetEnv(key, value string) error {
	cmd := newCommand(s.Client, "setenv", "-t", s.Target(), fmt.Sprint(key), fmt.Sprint(value))

	s.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SetHook registers a session-scoped hook command which tmux will run when the
// named hook fires.
func (s Session) SetHook(hook, command string) error {
	cmd := newCommand(s.Client, "set-hook", "-t", s.Target(), fmt.Sprint(hook), fmt.Sprint(command))

	s.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SetOption sets a session-scoped tmux option.
func (s Session) SetOption(option, value string) error {
	cmd := newCommand(s.Client, "set-option", "-t", s.Target(), fmt.Sprint(option), fmt.Sprint(value))

	s.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SendKeys sends the given keystrokes to the session's active pane without
// waiting for the command to complete. It is used for the final command in a
// sequence, which has no successor to gate and may be long-running or
// interactive (e.g. nvim, tail -f, a dev server).
func (s Session) SendKeys(keys string) error {
	cmd := newCommand(s.Client, "send", "-t", s.Target(), fmt.Sprint(keys), "Enter")

	s.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SendKeysAndWait sends the given keystrokes to the session's active pane and
// blocks until the command completes, using the same `tmux wait-for`
// synchronisation as Pane.SendKeysAndWait. Session-level commands target the
// session (and therefore its active pane) rather than a specific pane.
func (s Session) SendKeysAndWait(keys, channel string) error {
	signalled := fmt.Sprintf("%s ; tmux wait-for -S %s", keys, channel)

	cmd := newCommand(s.Client, "send", "-t", s.Target(), signalled, "Enter")

	s.logger.Debug(cmd.String())

	if err := cmd.Exec(); err != nil {
		return err
	}

	return s.Client.WaitFor(channel)
}
