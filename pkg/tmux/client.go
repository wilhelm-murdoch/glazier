package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/wilhelm-murdoch/go-collection"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

const (
	formatActiveSessions = "#{session_id};#{session_name};#{session_path}"
	formatActiveWindows  = "#{window_id};#{window_index};#{window_name};#{window_layout};#{window_active}"
	formatActivePanes    = "#{pane_id};#{pane_index};#{pane_title};#{pane_active};#{pane_current_path}"
)

// Client represents a tmux client.
type Client struct {
	CurrentSession *Session
	socketPath     string
	socketName     string
	logger         *slog.Logger
}

// NewClient returns a new client.
func NewClient(socketPath, socketName string, logger *slog.Logger) *Client {
	return &Client{
		socketPath: socketPath,
		socketName: socketName,
		logger:     logger,
	}
}

// Attach attaches to the given session. If we are inside a tmux session,
// we switch to the given session.
func (c *Client) Attach(session *Session) error {
	var args []string

	// Technically, you can specify both -L and -S parameters when creating
	// a tmux client session, but the last of the two will take precedence.
	if c.socketName != "" {
		args = append(args, "-L", c.socketName)
	}

	if c.socketPath != "" {
		args = append(args, "-S", c.socketPath)
	}

	if os.Getenv("TMUX") != "" {
		args = append(args, "attach", "-t", session.Target())
	} else {
		args = append(args, "switchc", "-t", session.Target())
	}

	cmd, err := newCommand(*c, args...)
	if err != nil {
		return err
	}

	c.logger.Debug(cmd.String())

	if err := cmd.Exec(); err != nil {
		if strings.Contains(err.Error(), "can't find session") {
			return fmt.Errorf(`session "%s" not found`, session.Name)
		}

		return err
	}

	c.CurrentSession = session

	return nil
}

// Sessions returns a collection of active sessions.
func (c Client) Sessions() (collection.Collection[*Session], error) {
	var sessions collection.Collection[*Session]

	args := []string{
		"ls",
		"-F",
		formatActiveSessions,
	}

	cmd, err := newCommand(c, args...)
	if err != nil {
		return sessions, err
	}

	c.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return sessions, err
	}

	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.SplitN(line, ";", 3)

		if len(parts) != 3 {
			return sessions, fmt.Errorf(
				"expected 3 parts for session line, but got %d instead: %s",
				len(parts),
				line,
			)
		}

		id, err := strconv.Atoi(strings.ReplaceAll(parts[0], "$", ""))
		if err != nil {
			return sessions, err
		}

		sessions.Push(&Session{
			Client:            c,
			Id:                id,
			Name:              strings.TrimSpace(parts[1]),
			StartingDirectory: strings.TrimSpace(parts[2]),
			logger:            c.logger,
		})
	}

	return sessions, err
}

// Windows returns a collection of windows for the given session.
func (c Client) Windows(session *Session) (collection.Collection[*Window], error) {
	var windows collection.Collection[*Window]

	args := []string{
		"lsw",
		"-F", formatActiveWindows,
		"-t", session.Target(),
	}

	cmd, err := newCommand(c, args...)
	if err != nil {
		return windows, err
	}

	c.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return windows, err
	}

	for window := range strings.SplitSeq(output, "\n") {
		parts := strings.SplitN(window, ";", 5)

		id, err := strconv.Atoi(strings.ReplaceAll(parts[0], "@", ""))
		if err != nil {
			return windows, err
		}

		index, err := strconv.Atoi(parts[1])
		if err != nil {
			return windows, err
		}

		windows.Push(&Window{
			Id:       id,
			Index:    index,
			Name:     parts[2],
			Layout:   enums.LayoutFromString(parts[3]),
			IsActive: parts[4] == "1",
			IsFirst:  parts[1] == "1",
			Session:  session,
		})
	}

	return windows, nil
}

// Panes returns a collection of panes for the given window.
func (c Client) Panes(window *Window) (collection.Collection[*Pane], error) {
	var panes collection.Collection[*Pane]

	args := []string{
		"lsp",
		"-F", formatActivePanes,
		"-t", window.Target(),
	}

	cmd, err := newCommand(c, args...)
	if err != nil {
		return panes, err
	}

	c.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return panes, err
	}

	baseIndexCmdParts, err := c.GetBaseIndex(window.Target(), "pane-base-index")
	if err != nil {
		return panes, err
	}

	if len(baseIndexCmdParts) != 2 {
		return panes, errors.New("could not determine pane base index")
	}

	for pane := range strings.SplitSeq(output, "\n") {
		parts := strings.Split(pane, ";")

		id, err := strconv.Atoi(strings.ReplaceAll(parts[0], "%", ""))
		if err != nil {
			return panes, err
		}

		index, err := strconv.Atoi(parts[1])
		if err != nil {
			return panes, err
		}

		panes.Push(&Pane{
			Id:                PaneId(id),
			Index:             index,
			Name:              parts[2],
			StartingDirectory: parts[4],
			IsActive:          parts[3] == "1",
			IsFirst:           parts[1] == baseIndexCmdParts[1],
			Window:            window,
		})
	}

	return panes, nil
}

// NewSession creates a new session with the given name and starting directory.
func (c Client) NewSession(sessionName, startingDirectory string) (*Session, error) {
	var session *Session

	cmd, err := newCommand(
		c,
		"new",
		"-d",
		"-s",
		fmt.Sprint(sessionName),
		"-c",
		fmt.Sprint(startingDirectory),
	)
	if err != nil {
		return session, err
	}

	c.logger.Debug(cmd.String())

	if err := cmd.Exec(); err != nil {
		return session, err
	}

	sessions, err := c.Sessions()
	if err != nil {
		return session, err
	}

	return sessions.Find(func(i int, s *Session) bool {
		return s.Name == fmt.Sprint(sessionName)
	}), nil
}

// NewSessionIfNotExists creates a new session with the given name and starting
// directory if it does not already exist.
func (c Client) NewSessionIfNotExists(sessionName, startingDirectory string) (*Session, error) {
	sessions, _ := c.Sessions()
	exists := sessions.Find(func(i int, s *Session) bool {
		return s.Name == fmt.Sprint(sessionName)
	})

	if exists == nil {
		return c.NewSession(sessionName, startingDirectory)
	}

	return exists, nil
}

// KillSession kills the given session.
func (c Client) KillSessionByName(sessionName string) error {
	cmd, _ := newCommand(c, "kill-session", "-t", fmt.Sprint(sessionName))

	if _, err := cmd.ExecWithOutput(); err != nil {
		return fmt.Errorf(`session "%s" could not be killed: %w`, sessionName, err)
	}

	c.logger.Debug(cmd.String())

	return nil
}

// FindSessionByName returns the session with the given name if it exists.
func (c Client) FindSessionByName(sessionName string) (*Session, error) {
	sessions, _ := c.Sessions()

	found := sessions.Find(func(i int, s *Session) bool {
		return s.Name == fmt.Sprint(sessionName)
	})

	if found != nil {
		return found, nil
	}

	return nil, fmt.Errorf(`session "%s" not found`, sessionName)
}

// HasSession returns true if a session with the given name exists.
func (c Client) HasSession(sessionName string) bool {
	cmd, _ := newCommand(c, "has-session", "-t", fmt.Sprint(sessionName))

	if exitStatus := cmd.ExecWithStatus(); exitStatus != 0 {
		return false
	}

	c.logger.Debug(cmd.String())

	return true
}

// GetOption returns the specified option for the target of the attached client session.
func (c Client) GetOption(target, option string) (string, error) {
	// Currently supports looking for globally-set options, but can be extended for additional
	// targeted flags for sessions, windows and panes if needed.
	args := []string{
		"show",
		"-g",
		"-t",
		target,
		option,
	}

	cmd, err := newCommand(c, args...)
	if err != nil {
		return "", err
	}

	c.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return "", err
	}

	return output, nil
}

// GetBaseIndex is a helper method which attempts to return the base index option
// for the specified target which may be derived from a Window or a Pane.
func (c Client) GetBaseIndex(target, option string) ([]string, error) {
	var out []string

	result, err := c.GetOption(target, option)
	if err != nil {
		return out, err
	}

	return strings.Split(result, " "), nil
}
