package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/wilhelm-murdoch/go-collection"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
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

	if !IsInsideTmux() {
		args = append(args, "attach", "-t", session.Target())
	} else {
		args = append(args, "switchc", "-t", session.Target())
	}

	cmd, err := NewCommand(*c, args...)

	c.logger.Debug(cmd.String())
	if err != nil {
		return err
	}

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

	format := []string{
		"#{session_id}",
		"#{session_name}",
		"#{session_path}",
	}

	args := []string{
		"ls",
		"-F", strings.Join(format, ";"),
	}

	cmd, err := NewCommand(c, args...)

	c.logger.Debug(cmd.String())
	if err != nil {
		return sessions, err
	}

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return sessions, err
	}

	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.SplitN(line, ";", 3)

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

	format := []string{
		"#{window_id}",
		"#{window_index}",
		"#{window_name}",
		"#{window_layout}",
		"#{window_active}",
	}

	args := []string{
		"lsw",
		"-F", strings.Join(format, ";"),
		"-t", session.Target(),
	}

	cmd, err := NewCommand(c, args...)

	c.logger.Debug(cmd.String())
	if err != nil {
		return windows, err
	}

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return windows, err
	}

	// baseIndex, err := getOption[enums.OptionsSession](c, "show", "-g", "-t", session.Target(), enums.OptionsSessionBaseIndexString)
	// if err != nil {
	// 	return windows, err
	// }

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
			// IsFirst:  parts[1] == baseIndex.Value,
			Session: session,
		})
	}

	return windows, nil
}

// Panes returns a collection of panes for the given window.
func (c Client) Panes(window *Window) (collection.Collection[*Pane], error) {
	var panes collection.Collection[*Pane]

	format := []string{
		"#{pane_id}",
		"#{pane_index}",
		"#{pane_title}",
		"#{pane_active}",
		"#{pane_current_path}",
	}

	args := []string{
		"lsp",
		"-F", strings.Join(format, ";"),
		"-t", window.Target(),
	}

	cmd, err := NewCommand(c, args...)

	c.logger.Debug(cmd.String())
	if err != nil {
		return panes, err
	}

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return panes, err
	}

	args = []string{
		"show",
		"-gw",
		"-t", window.Target(),
		"pane-base-index",
	}

	baseIndexCmd, err := NewCommand(c, args...)

	c.logger.Debug(cmd.String())
	if err != nil {
		return panes, err
	}

	baseIndexCmdOutput, err := baseIndexCmd.ExecWithOutput()
	if err != nil {
		return panes, err
	}

	baseIndexCmdParts := strings.Split(baseIndexCmdOutput, " ")
	if len(baseIndexCmdParts) != 2 {
		return panes, errors.New("could not determine global base index")
	}

	for pane := range strings.SplitSeq(output, "\n") {
		parts := strings.SplitN(pane, ";", len(format))

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

	cmd, err := NewCommand(
		c,
		"new",
		"-d",
		"-s",
		fmt.Sprint(sessionName),
		"-c",
		fmt.Sprint(startingDirectory),
	)

	c.logger.Debug(cmd.String())
	if err != nil {
		return session, err
	}

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
	cmd, _ := NewCommand(c, "kill-session", "-t", fmt.Sprint(sessionName))

	c.logger.Debug(cmd.String())
	if _, err := cmd.ExecWithOutput(); err != nil {
		return fmt.Errorf("%w", err)
	}

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
	cmd, _ := NewCommand(c, "has-session", "-t", fmt.Sprint(sessionName))

	c.logger.Debug(cmd.String())
	if exitStatus := cmd.ExecWithStatus(); exitStatus != 0 {
		return false
	}

	return true
}
