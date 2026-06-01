package tmux

import (
	"fmt"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

type PaneId int

// String is responsible for returning the string representation of the PaneId.
func (id PaneId) String() string {
	return fmt.Sprintf("%%%d", int(id))
}

// Pane represents a tmux pane.
type Pane struct {
	Window            *Window
	Name              string
	StartingDirectory string
	IsActive          bool
	IsFirst           bool
	Index             int
	Id                PaneId
}

// Target returns the target pane by its composite id of session name, window id, and pane id.
func (p Pane) Target() string {
	return fmt.Sprintf(`%s:%d.%d`, p.Window.Session.Name, p.Window.Index, p.Index)
}

// SendKeys sends the given keystrokes to the current pane.
func (p Pane) SendKeys(keys string) error {
	args := []string{
		"send",
		"-t",
		p.Target(),
		fmt.Sprint(keys),
		"Enter",
	}

	cmd := newCommand(p.Window.Session.Client, args...)

	p.Window.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SendKeysAndWait sends the given keystrokes to the current pane and blocks
// until the command completes. It appends a `tmux wait-for -S <channel>` signal
// to the command and then waits on the same channel, replacing fixed sleeps
// with reliable synchronisation. The signal is recorded by the tmux server even
// if it arrives before the wait begins, so there is no race between sending the
// command and waiting on its completion.
func (p Pane) SendKeysAndWait(keys, channel string) error {
	signalled := fmt.Sprintf("%s ; tmux wait-for -S %s", keys, channel)

	if err := p.SendKeys(signalled); err != nil {
		return err
	}

	return p.Window.Session.Client.WaitFor(channel)
}

// SetEnv sets the given environment variable to the given value on the session
// that owns this pane. tmux scopes environment variables to sessions, so the
// target is the owning session rather than the pane itself.
func (p Pane) SetEnv(key, value string) error {
	args := []string{
		"setenv",
		"-t",
		p.Window.Session.Target(),
		fmt.Sprint(key),
		fmt.Sprint(value),
	}

	cmd := newCommand(p.Window.Session.Client, args...)

	p.Window.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SetHook registers a pane-scoped hook command which tmux will run when the
// named hook fires for this pane.
func (p Pane) SetHook(hook, command string) error {
	cmd := newCommand(
		p.Window.Session.Client,
		"set-hook",
		"-p",
		"-t", p.Target(),
		fmt.Sprint(hook),
		fmt.Sprint(command),
	)

	p.Window.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// Resize is responsible for modifying the height, or width, of the current pane.
func (p Pane) Resize(x, y string) error {
	args := []string{
		"resizep",
		"-t", p.Target(),
		"-x", x,
		"-y", y,
	}

	cmd := newCommand(p.Window.Session.Client, args...)

	p.Window.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SetOption sets a pane-scoped tmux option.
func (p Pane) SetOption(option, value string) error {
	cmd := newCommand(
		p.Window.Session.Client,
		"set-option",
		"-p",
		"-t", p.Target(),
		fmt.Sprint(option),
		fmt.Sprint(value),
	)

	p.Window.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// Adjust resizes the pane in the given direction by the given amount using
// `tmux resize-pane -U|-D|-L|-R`. An unknown direction yields an error.
func (p Pane) Adjust(direction enums.Adjustment, amount string) error {
	flag, ok := direction.ResizeFlag()
	if !ok {
		return fmt.Errorf("unknown pane adjustment direction `%s`", direction)
	}

	cmd := newCommand(
		p.Window.Session.Client,
		"resizep",
		"-t", p.Target(),
		flag,
		fmt.Sprint(amount),
	)

	p.Window.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// Select is responsible for selecting the current pane.
func (p Pane) Select() error {
	cmd := newCommand(p.Window.Session.Client, "selectp", "-t", p.Target())

	p.Window.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// Kill closes the current pane.
func (p Pane) Kill() error {
	cmd := newCommand(p.Window.Session.Client, "killp", "-t", p.Target())

	p.Window.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}
