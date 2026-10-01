package tmux

import (
	"fmt"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

type PaneId int

// String returns the tmux id of the pane, for example %7.
func (id PaneId) String() string {
	return fmt.Sprintf("%%%d", int(id))
}

// Pane is a tmux pane.
type Pane struct {
	Window            *Window
	Name              string
	StartingDirectory string
	IsActive          bool
	Index             int
	Id                PaneId
}

// Target returns the id that tmux commands use to address the pane.
func (p Pane) Target() string {
	return p.Id.String()
}

// client returns the client of the server that holds the pane.
func (p Pane) client() Client {
	return p.Window.Session.Client
}

// SetHook sets a pane hook.
func (p Pane) SetHook(hook, command string) error {
	return p.client().setScoped("set-hook", "-p", p.Target(), hook, command)
}

// SetOption sets a pane option.
func (p Pane) SetOption(option, value string) error {
	return p.client().setScoped("set-option", "-p", p.Target(), option, value)
}

// Resize sets the width, the height or both, in cells or as a percentage. An empty x or y keeps that axis.
func (p Pane) Resize(x, y string) error {
	args := []string{"resizep", "-t", p.Target()}
	if x != "" {
		args = append(args, "-x", x)
	}

	if y != "" {
		args = append(args, "-y", y)
	}

	return p.client().run(args...)
}

// Adjust grows or shrinks the pane in the given direction by amount cells.
func (p Pane) Adjust(direction enums.Adjustment, amount string) error {
	flag, ok := direction.ResizeFlag()
	if !ok {
		return fmt.Errorf("unknown pane adjustment direction `%s`", direction)
	}

	return p.client().run("resizep", "-t", p.Target(), flag, amount)
}

// Select makes the pane the active pane of its window.
func (p Pane) Select() error {
	return p.client().run("selectp", "-t", p.Target())
}

// Kill kills the pane.
func (p Pane) Kill() error {
	return p.client().run("killp", "-t", p.Target())
}
