package tmux

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/wilhelm-murdoch/glazier/internal/tmux/enums"
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
	Id                int
}

// Target returns the target session by its name.
func (s *Session) Target() string {
	return s.Name
}

// NewWindow creates a new window in the current session and returns it.
func (s *Session) NewWindow(windowName string) (*Window, error) {
	var window *Window

	format := []string{
		"#{window_id}",
		"#{window_index}",
		"#{window_name}",
		"#{window_layout}",
		"#{window_active}",
	}

	args := []string{
		"neww",
		"-d",
		"-t", s.Name,
		"-n", fmt.Sprint(windowName),
		"-F", strings.Join(format, ";"),
		"-P",
	}

	cmd, err := NewCommand(s.Client, args...)
	if err != nil {
		return window, err
	}

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return window, err
	}

	parts := strings.SplitN(output, ";", len(format))

	id, err := strconv.Atoi(strings.ReplaceAll(parts[0], "@", ""))
	if err != nil {
		return window, err
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil {
		return window, err
	}

	args = []string{
		"show",
		"-g",
		"-t", s.Target(),
		"base-index",
	}

	baseIndexCmd, err := NewCommand(s.Client, args...)
	if err != nil {
		return window, err
	}

	baseIndexCmdOutput, err := baseIndexCmd.ExecWithOutput()
	if err != nil {
		return window, err
	}

	baseIndexCmdParts := strings.Split(baseIndexCmdOutput, " ")
	if len(baseIndexCmdParts) != 2 {
		return window, errors.New("could not determine window base index")
	}

	return &Window{
		Id:       id,
		Index:    index,
		Name:     parts[2],
		Layout:   enums.LayoutFromString(parts[3]),
		IsActive: parts[4] == "1",
		IsFirst:  parts[1] == baseIndexCmdParts[1],
		Session:  s,
	}, nil
}

// Kill closes the current session.
func (s *Session) Kill() error {
	cmd, err := NewCommand(s.Client, "kill-session", "-t", s.Target())
	if err != nil {
		return err
	}

	return cmd.Exec()
}
