package tmux

import (
	"fmt"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

type WindowId int

// String returns the tmux id of the window, for example @5.
func (id WindowId) String() string {
	return fmt.Sprintf("@%d", int(id))
}

// Window is a tmux window.
type Window struct {
	Session  *Session
	Name     string
	IsActive bool
	Id       WindowId
	Index    int
	Layout   enums.Layout

	// RawLayout is the #{window_layout} coordinate string, which `save` keeps when no named preset matches.
	RawLayout string
}

// Target returns the id that tmux commands use to address the window.
func (w Window) Target() string {
	return w.Id.String()
}

// Split creates a pane from the parent pane and gives it the name as its title.
func (w *Window) Split(parentId, name, startingDirectory string) (*Pane, error) {
	client := w.Session.Client
	name = SanitizeName(name)

	output, err := client.output(
		"splitw", "-Pd",
		"-t", parentId,
		"-c", escapeFormat(startingDirectory),
		"-F", formatActivePanes,
	)
	if err != nil {
		return nil, err
	}

	pane, err := client.NewPaneFromLine(output, w)
	if err != nil {
		return nil, err
	}

	// tmux can report an empty path for a pane that has only just started.
	if pane.StartingDirectory == "" {
		pane.StartingDirectory = startingDirectory
	}

	if err := client.run("selectp", "-T", escapeFormat(name), "-t", pane.Target()); err != nil {
		return pane, err
	}

	// Right after the split, tmux reports the host name as the title, so keep the name from the profile.
	pane.Name = name

	return pane, nil
}

// Rename gives the window a new name, sanitised and escaped like a name passed to NewWindow.
func (w *Window) Rename(name string) error {
	name = SanitizeName(name)

	if err := w.Session.Client.run("renamew", "-t", w.Target(), escapeFormat(name)); err != nil {
		return err
	}

	w.Name = name

	return nil
}

// Select makes the window the active window of its session.
func (w Window) Select() error {
	return w.Session.Client.run("selectw", "-t", w.Target())
}

// SelectLayout applies a named preset or a raw layout string; tmux rejects a raw string with a bad checksum.
func (w Window) SelectLayout(layout string) error {
	return w.Session.Client.run("selectl", "-t", w.Target(), layout)
}

// SetHook sets a window hook.
func (w Window) SetHook(hook, command string) error {
	return w.Session.Client.setScoped("set-hook", "-w", w.Target(), hook, command)
}

// SetOption sets a window option.
func (w Window) SetOption(option, value string) error {
	return w.Session.Client.setScoped("set-option", "-w", w.Target(), option, value)
}
