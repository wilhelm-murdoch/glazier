package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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

var defaultTmuxExecutablePath = "tmux"

// Client represents a tmux client.
type Client struct {
	CurrentSession *Session
	socketPath     string
	socketName     string
	logger         *slog.Logger
	tmuxPath       string
}

// NewClient returns a new client.
func NewClient(socketPath, socketName string, logger *slog.Logger) (*Client, error) {
	resolvedTmuxPath, err := exec.LookPath(defaultTmuxExecutablePath)
	if err != nil {
		return nil, fmt.Errorf("tmux is not installed")
	}

	return &Client{
		socketPath: socketPath,
		socketName: socketName,
		logger:     logger,
		tmuxPath:   resolvedTmuxPath,
	}, nil
}

// IsRunning returns true if the local tmux server is currently running.
func (c Client) IsRunning() bool {
	cmd := newCommand(c, "info")

	c.logger.Debug(cmd.String())

	if exitStatus := cmd.ExecWithStatus(); exitStatus == 0 {
		return true
	}

	return false
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

	cmd := newCommand(*c, args...)

	c.logger.Debug(cmd.String())

	if err := cmd.Exec(); err != nil {
		return fmt.Errorf(
			`attaching to session "%s" yielded the following error from the client: %w`,
			session.Name,
			err,
		)
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

	cmd := newCommand(c, args...)

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

	cmd := newCommand(c, args...)

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

	cmd := newCommand(c, args...)

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

	args := []string{
		"new",
		"-d",
		"-s",
		fmt.Sprint(sessionName),
		"-c",
		fmt.Sprint(startingDirectory),
	}

	cmd := newCommand(c, args...)

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
	cmd := newCommand(c, "kill-session", "-t", fmt.Sprint(sessionName))

	c.logger.Debug(cmd.String())

	if _, err := cmd.ExecWithOutput(); err != nil {
		return fmt.Errorf(`session "%s" could not be killed: %w`, sessionName, err)
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
	cmd := newCommand(c, "has-session", "-t", fmt.Sprint(sessionName))

	c.logger.Debug(cmd.String())

	if exitStatus := cmd.ExecWithStatus(); exitStatus != 0 {
		return false
	}

	return true
}

// GetOption returns the specified option for the target of the attached client session.
func (c Client) GetOption(target, option, scope string) (string, error) {
	scopes := map[string]string{
		"global":  "-g",
		"pane":    "-p",
		"window":  "-w",
		"session": "-s",
	}

	resolvedScope, ok := scopes[scope]
	if !ok {
		c.logger.Warn(
			"get option scope `%s` could not be resolved for target `%s`, option `%s`; reverting to server scope instead",
		)
		resolvedScope = ""
	}

	args := []string{
		"show",
		resolvedScope,
		"-t",
		target,
		option,
	}

	cmd := newCommand(c, args...)

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

	// TODO: update this function to accept various scopes; defaulting to global for now
	result, err := c.GetOption(target, option, "")
	if err != nil {
		return out, err
	}

	return strings.Split(result, " "), nil
}
