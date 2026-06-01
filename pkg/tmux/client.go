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

// IsRunning returns true if the local tmux server is currently running. It uses
// `list-sessions` rather than `server-info` (`info`): the latter requires an
// attached client and exits non-zero with "no current client" when a server is
// running but only has detached sessions (e.g. in CI), which is a false
// negative. `list-sessions` talks to the server without needing a client and
// exits 0 whenever the server is up.
func (c Client) IsRunning() bool {
	cmd := newCommand(c, "list-sessions")

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
		args = append(args, "switchc", "-t", session.Target())
	} else {
		args = append(args, "attach", "-t", session.Target())
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
		session, err := c.NewSessionFromLine(line)
		if err != nil {
			return sessions, err
		}

		sessions.Push(session)
	}

	return sessions, err
}

func (c Client) NewSessionFromLine(line string) (*Session, error) {
	parts, id, err := c.getPartsFromTmuxLine(line, "$", 3)
	if err != nil {
		return nil, err
	}

	return &Session{
		Client:            c,
		Id:                id,
		Name:              strings.TrimSpace(parts[1]),
		StartingDirectory: strings.TrimSpace(parts[2]),
		logger:            c.logger,
	}, nil
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

	for line := range strings.SplitSeq(output, "\n") {
		window, err := c.NewWindowFromLine(line, session)
		if err != nil {
			return windows, err
		}

		windows.Push(window)
	}

	return windows, nil
}

func (c Client) NewWindowFromLine(line string, session *Session) (*Window, error) {
	parts, id, err := c.getPartsFromTmuxLine(line, "@", 5)
	if err != nil {
		return nil, err
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}

	return &Window{
		Id:       id,
		Index:    index,
		Name:     parts[2],
		Layout:   enums.LayoutFromString(parts[3]),
		IsActive: parts[4] == "1",
		IsFirst:  parts[1] == "1",
		Session:  session,
	}, nil
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

	for line := range strings.SplitSeq(output, "\n") {
		pane, err := c.NewPaneFromLine(line, baseIndexCmdParts[1], window)
		if err != nil {
			return panes, err
		}

		panes.Push(pane)
	}

	return panes, nil
}

func (c Client) NewPaneFromLine(line, baseIndex string, window *Window) (*Pane, error) {
	parts, id, err := c.getPartsFromTmuxLine(line, "%", 5)
	if err != nil {
		return nil, err
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}

	return &Pane{
		Id:                PaneId(id),
		Index:             index,
		Name:              parts[2],
		StartingDirectory: parts[4],
		IsActive:          parts[3] == "1",
		IsFirst:           parts[1] == baseIndex,
		Window:            window,
	}, nil
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

	session = sessions.Find(func(i int, s *Session) bool {
		return s.Name == fmt.Sprint(sessionName)
	})

	if session == nil {
		return nil, fmt.Errorf(
			"session `%s` was created but could not be found afterwards",
			sessionName,
		)
	}

	return session, nil
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

// CurrentSessionName returns the name of the session attached to the current
// client. This is intended to be called from within a running tmux session
// (e.g. by the `save` command) to determine which session to capture.
func (c Client) CurrentSessionName() (string, error) {
	cmd := newCommand(c, "display-message", "-p", "#{session_name}")

	c.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return "", fmt.Errorf("could not determine current session: %w", err)
	}

	return strings.TrimSpace(output), nil
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
			"get option scope could not be resolved; reverting to global scope instead",
			"scope", scope,
			"target", target,
			"option", option,
		)
		resolvedScope = "-g"
	}

	// Note: an empty scope flag must not be appended, since tmux treats an empty
	// argument as an extra positional and rejects the command.
	args := []string{"show", resolvedScope, "-t", target, option}

	cmd := newCommand(c, args...)

	c.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return "", err
	}

	return output, nil
}

// WaitFor blocks until the given channel is signalled with `tmux wait-for -S`,
// which is used to serialise command execution within a pane. The tmux server
// records the signal even if it is sent before the wait begins, so there is no
// race between dispatching a command and waiting on its completion.
func (c Client) WaitFor(channel string) error {
	cmd := newCommand(c, "wait-for", channel)

	c.logger.Debug(cmd.String())

	if err := cmd.Exec(); err != nil {
		return fmt.Errorf("waiting on channel `%s` failed: %w", channel, err)
	}

	return nil
}

// GetBaseIndex is a helper method which attempts to return the base index option
// for the specified target which may be derived from a Window or a Pane.
func (c Client) GetBaseIndex(target, option string) ([]string, error) {
	var out []string

	// base-index / pane-base-index are typically configured globally; querying
	// at global scope reliably returns the effective value (a session-scoped
	// query only reports values explicitly set on that session).
	result, err := c.GetOption(target, option, "global")
	if err != nil {
		return out, err
	}

	return strings.Split(result, " "), nil
}

func (c Client) getPartsFromTmuxLine(
	line, prefix string,
	expectedLength int,
) ([]string, int, error) {
	parts := strings.SplitN(line, ";", expectedLength)

	if len(parts) != expectedLength {
		return parts, 0, fmt.Errorf(
			"expected %d parts for tmux line, but got %d instead: %s",
			expectedLength,
			len(parts),
			line,
		)
	}

	id, err := strconv.Atoi(strings.ReplaceAll(parts[0], prefix, ""))
	if err != nil {
		return parts, 0, err
	}

	return parts, id, nil
}
