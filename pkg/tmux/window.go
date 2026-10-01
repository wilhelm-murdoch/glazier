package tmux

import (
	"fmt"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

type WindowId int

// String is responsible for returning the string representation of the WindowId.
func (id WindowId) String() string {
	return fmt.Sprintf("@%d", int(id))
}

// Window represents a tmux window.
type Window struct {
	Session  *Session
	Name     string
	IsActive bool
	Id       WindowId
	Index    int
	Layout   enums.Layout
	// RawLayout is the verbatim tmux window layout coordinate string (the
	// #{window_layout} value). It is preserved so `save` can capture a layout
	// that does not map to a named preset.
	RawLayout string
}

// Target returns the target window by its string representation of the WindowId.
func (w Window) Target() string {
	return w.Id.String()
}

// Split splits the current window into two panes.
func (w *Window) Split(parentId, name, startingDirectory string) (*Pane, error) {
	var pane *Pane

	name = SanitizeName(name)

	args := []string{
		"splitw",
		"-Pd",
		"-t", parentId,
		"-c", escapeFormat(startingDirectory),
		"-F", formatActivePanes,
	}

	cmd := newCommand(w.Session.Client, args...)

	w.Session.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return pane, err
	}

	pane, err = w.Session.Client.NewPaneFromLine(output, w)
	if err != nil {
		return pane, err
	}

	// tmux can report an empty path for a pane that has only just started.
	if pane.StartingDirectory == "" {
		pane.StartingDirectory = startingDirectory
	}

	cmd = newCommand(w.Session.Client, "selectp", "-T", escapeFormat(name), "-t", pane.Id.String())

	w.Session.logger.Debug(cmd.String())

	if err = cmd.Exec(); err != nil {
		return pane, err
	}

	// The name tmux gives us immediately after the split is the name of the host.
	// Explicitly reset the name to the one derived from the associated glaze file.
	pane.Name = name

	return pane, nil
}

// Rename gives the window a new name, sanitised and escaped like a name passed to NewWindow.
func (w *Window) Rename(name string) error {
	name = SanitizeName(name)

	cmd := newCommand(w.Session.Client, "renamew", "-t", w.Target(), escapeFormat(name))

	w.Session.logger.Debug(cmd.String())

	if err := cmd.Exec(); err != nil {
		return err
	}

	w.Name = name

	return nil
}

// Select is responsible for selecting the current window.
func (w Window) Select() error {
	cmd := newCommand(w.Session.Client, "selectw", "-t", w.Target())

	w.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SelectLayout is responsible for selecting the layout for the current window.
// The layout is either a named preset (e.g. "tiled") or a raw tmux layout
// coordinate string; tmux's select-layout accepts both. An invalid or
// checksum-stale coordinate string is rejected by tmux here, failing the `up`.
func (w Window) SelectLayout(layout string) error {
	cmd := newCommand(w.Session.Client, "selectl", "-t", w.Target(), layout)

	w.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SetHook registers a window-scoped hook command which tmux will run when the
// named hook fires for this window.
func (w Window) SetHook(hook, command string) error {
	cmd := newCommand(w.Session.Client, "set-hook", "-w", "-t", w.Target(), fmt.Sprint(hook), fmt.Sprint(command))

	w.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}

// SetOption sets a window-scoped tmux option.
func (w Window) SetOption(option, value string) error {
	cmd := newCommand(w.Session.Client, "set-option", "-w", "-t", w.Target(), fmt.Sprint(option), fmt.Sprint(value))

	w.Session.logger.Debug(cmd.String())

	return cmd.Exec()
}
