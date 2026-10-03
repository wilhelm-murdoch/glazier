package tmux

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

var defaultTmuxExecutablePath = "tmux"

// Client runs tmux commands against one tmux server.
type Client struct {
	socketPath string
	socketName string
	logger     *slog.Logger
	tmuxPath   string

	// ctx stops running tmux commands when it is cancelled. A nil ctx never cancels.
	ctx context.Context
}

// NewClient returns a client for the server on socketPath or socketName, or on the default socket when both are empty.
func NewClient(socketPath, socketName string, logger *slog.Logger) (Client, error) {
	resolvedTmuxPath, err := exec.LookPath(defaultTmuxExecutablePath)
	if err != nil {
		return Client{}, fmt.Errorf("%w: tmux is not installed or not on PATH", ErrUnreachable)
	}

	return Client{
		socketPath: socketPath,
		socketName: socketName,
		logger:     logger,
		tmuxPath:   resolvedTmuxPath,
	}, nil
}

// WithContext returns a copy of the client whose tmux commands stop when ctx is cancelled.
func (c Client) WithContext(ctx context.Context) Client {
	c.ctx = ctx
	return c
}

// WithoutCancel returns a copy of the client whose commands still run after a cancellation, for clean-up.
func (c Client) WithoutCancel() Client {
	c.ctx = context.WithoutCancel(c.context())
	return c
}

// context returns the context of the client, or a context that never cancels.
func (c Client) context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}

	return c.ctx
}

// run runs a tmux command whose output glaze does not need.
func (c Client) run(args ...string) error {
	return newCommand(c, args...).Exec()
}

// output runs a tmux command and returns what it printed.
func (c Client) output(args ...string) (string, error) {
	return newCommand(c, args...).ExecWithOutput()
}

// setScoped runs set-hook or set-option on target. The scope flag is "" for a session, "-w" for a window and "-p" for a pane.
// "--" ends the flags, so a name or a value that starts with "-" stays an operand.
func (c Client) setScoped(command, scope, target, name, value string) error {
	args := []string{command}
	if scope != "" {
		args = append(args, scope)
	}

	return c.run(append(args, "-t", target, "--", name, value)...)
}

// parseLines parses each line of a tmux listing with parse. Empty output, for example from a server with no sessions, has no items.
func parseLines[T any](output string, parse func(line string) (*T, error)) ([]*T, error) {
	var items []*T
	if output == "" {
		return items, nil
	}

	for line := range strings.SplitSeq(output, "\n") {
		item, err := parse(line)
		if err != nil {
			return items, err
		}

		items = append(items, item)
	}

	return items, nil
}

// Sessions returns the sessions on the server.
func (c Client) Sessions() ([]*Session, error) {
	output, err := c.output("ls", "-F", formatActiveSessions)
	if err != nil {
		return nil, err
	}

	return parseLines(output, c.NewSessionFromLine)
}

// NewSessionFromLine parses one line of formatActiveSessions output.
func (c Client) NewSessionFromLine(line string) (*Session, error) {
	parts, id, err := getPartsFromTmuxLine(line, "$", 3)
	if err != nil {
		return nil, err
	}

	return &Session{
		Client:            c,
		Id:                SessionId(id),
		Name:              strings.TrimSpace(parts[1]),
		StartingDirectory: strings.TrimSpace(parts[2]),
	}, nil
}

// Windows returns the windows of the given session.
func (c Client) Windows(session *Session) ([]*Window, error) {
	output, err := c.output("lsw", "-F", formatActiveWindows, "-t", session.Target())
	if err != nil {
		return nil, err
	}

	return parseLines(output, func(line string) (*Window, error) {
		return c.NewWindowFromLine(line, session)
	})
}

// NewWindowFromLine parses one line of formatActiveWindows output.
func (c Client) NewWindowFromLine(line string, session *Session) (*Window, error) {
	parts, id, err := getPartsFromTmuxLine(line, "@", 5)
	if err != nil {
		return nil, err
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}

	return &Window{
		Id:        WindowId(id),
		Index:     index,
		Name:      parts[2],
		Layout:    enums.LayoutFromString(parts[3]),
		RawLayout: parts[3],
		IsActive:  parts[4] == "1",
		Session:   session,
	}, nil
}

// Panes returns the panes of the given window.
func (c Client) Panes(window *Window) ([]*Pane, error) {
	output, err := c.output("lsp", "-F", formatActivePanes, "-t", window.Target())
	if err != nil {
		return nil, err
	}

	return parseLines(output, func(line string) (*Pane, error) {
		return c.NewPaneFromLine(line, window)
	})
}

// NewPaneFromLine parses one line of formatActivePanes output.
func (c Client) NewPaneFromLine(line string, window *Window) (*Pane, error) {
	parts, id, err := getPartsFromTmuxLine(line, "%", 5)
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
		Window:            window,
	}, nil
}

// NewSession creates a detached session with the given name and starting directory.
func (c Client) NewSession(sessionName, startingDirectory string) (*Session, error) {
	output, err := c.output(
		"new", "-d",
		"-s", escapeFormat(SanitizeSessionName(sessionName)),
		"-c", escapeFormat(startingDirectory),
		"-F", formatActiveSessions, "-P",
	)
	if tmuxSaid(err, "duplicate session") {
		return nil, fmt.Errorf("%w: %w", ErrDuplicateSession, err)
	}

	if err != nil {
		return nil, err
	}

	return c.NewSessionFromLine(output)
}

// KillSessionByName kills the session with exactly the given name; "=" stops tmux matching a prefix.
func (c Client) KillSessionByName(sessionName string) error {
	sessionName = SanitizeSessionName(sessionName)

	if _, err := c.output("kill-session", "-t", "="+sessionName); err != nil {
		return fmt.Errorf("session `%s` could not be killed: %w", sessionName, err)
	}

	return nil
}

// FindSessionByName returns the session with exactly the given name.
func (c Client) FindSessionByName(sessionName string) (*Session, error) {
	sessionName = SanitizeSessionName(sessionName)

	sessions, err := c.Sessions()
	if err != nil {
		return nil, fmt.Errorf("could not list sessions: %w", err)
	}

	index := slices.IndexFunc(sessions, func(s *Session) bool { return s.Name == sessionName })
	if index == -1 {
		return nil, fmt.Errorf("session `%s` not found", sessionName)
	}

	return sessions[index], nil
}
