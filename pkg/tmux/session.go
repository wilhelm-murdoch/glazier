package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

const formatNewWindowResponse = "#{window_id};#{window_index};#{window_name};#{window_layout};#{window_active}"

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
	logger            *slog.Logger
}

// Target returns the target session by its name.
func (s Session) Target() string {
	return s.Name
}

// NewWindow creates a new window in the current session and returns it.
func (s *Session) NewWindow(windowName string) (*Window, error) {
	var window *Window

	args := []string{
		"neww",
		"-d",
		"-t", s.Name,
		"-n", fmt.Sprint(windowName),
		"-F", formatNewWindowResponse,
		"-P",
	}

	cmd, err := NewCommand(s.Client, args...)

	s.logger.Debug(cmd.String())
	if err != nil {
		return window, err
	}

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return window, err
	}

	parts := strings.Split(output, ";")

	id, err := strconv.Atoi(strings.ReplaceAll(parts[0], "@", ""))
	if err != nil {
		return window, err
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil {
		return window, err
	}

	baseIndexCmdParts, err := s.Client.GetBaseIndex(s.Target(), "base-index")
	if err != nil {
		return window, err
	}

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
func (s Session) Kill() error {
	cmd, err := NewCommand(s.Client, "kill-session", "-t", s.Target())

	s.logger.Debug(cmd.String())
	if err != nil {
		return err
	}

	return cmd.Exec()
}
