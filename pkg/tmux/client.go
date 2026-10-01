package tmux

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

var defaultTmuxExecutablePath = "tmux"

var (
	// ErrUnreachable means that glaze cannot run tmux or cannot connect to the tmux server.
	ErrUnreachable = errors.New("tmux is unreachable")

	// ErrOtherServer means that glaze runs inside a different tmux server, so attaching would nest a client in a pane.
	ErrOtherServer = errors.New("glaze runs inside a different tmux server")

	// ErrDuplicateSession means that another client created a session with the same name first.
	ErrDuplicateSession = errors.New("a session with this name already exists")
)

// Client represents a tmux client.
type Client struct {
	socketPath string
	socketName string
	logger     *slog.Logger
	tmuxPath   string

	// ctx stops running tmux commands when it is cancelled. A nil ctx never cancels.
	ctx context.Context
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

// NewClient returns a new client.
func NewClient(socketPath, socketName string, logger *slog.Logger) (*Client, error) {
	resolvedTmuxPath, err := exec.LookPath(defaultTmuxExecutablePath)
	if err != nil {
		return nil, fmt.Errorf("%w: tmux is not installed or not on PATH", ErrUnreachable)
	}

	return &Client{
		socketPath: socketPath,
		socketName: socketName,
		logger:     logger,
		tmuxPath:   resolvedTmuxPath,
	}, nil
}

// IsRunning reports whether a tmux server runs on the socket; `server-info` would need an attached client, so it uses `list-sessions`.
// An error means that glaze cannot reach the server, which is not the same as no server.
func (c Client) IsRunning() (bool, error) {
	cmd := newCommand(c, "list-sessions")

	c.logger.Debug(cmd.String())

	if _, err := cmd.ExecWithOutput(); err != nil {
		return false, lookupFailure(err)
	}

	return true, nil
}

// absentOutputs are the messages with which tmux says that the server or the session does not exist.
// A missing socket file and a missing socket directory both give "No such file or directory".
var absentOutputs = []string{"can't find session", "no server running on", "(No such file or directory)"}

// lookupFailure returns nil when err only says that the server or the session does not exist.
// Any other failure, for example a socket without permission, means that glaze cannot reach tmux.
func lookupFailure(err error) error {
	var withOutput CommandErrorWithOutput
	if errors.As(err, &withOutput) {
		for _, output := range absentOutputs {
			if strings.Contains(withOutput.Output, output) {
				return nil
			}
		}
	}

	return fmt.Errorf("%w: %w", ErrUnreachable, err)
}

// Attach attaches the terminal to the session, or switches the current client when glaze runs inside this server.
// It returns ErrOtherServer when glaze runs inside another tmux server, because attaching there would nest a client.
func (c *Client) Attach(session *Session) error {
	inside, err := c.InsideServer()
	if err != nil {
		return err
	}

	// NewCommand adds the socket flags.
	args := []string{"attach", "-t", session.Target()}
	switch {
	case inside:
		args = []string{"switchc", "-t", session.Target()}
	case os.Getenv("TMUX") != "":
		return ErrOtherServer
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

	return nil
}

// sessionNameReplacer replaces the characters that tmux rewrites only in session names.
// `$` is replaced on every version so that one profile gives the same name everywhere.
var sessionNameReplacer = strings.NewReplacer(".", "-", ":", "-", "$", "-")

// SanitizeName replaces each backslash and control character with a hyphen, because tmux 3.7 and later store them escaped.
func SanitizeName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '\\' || unicode.IsControl(r) {
			return '-'
		}

		return r
	}, name)
}

// SanitizeSessionName returns a session name that tmux stores unchanged.
func SanitizeSessionName(sessionName string) string {
	return SanitizeName(sessionNameReplacer.Replace(sessionName))
}

// findSessionByName returns the first session with the given name, or nil.
func findSessionByName(sessions []*Session, sessionName string) *Session {
	index := slices.IndexFunc(sessions, func(s *Session) bool {
		return s.Name == sessionName
	})

	if index == -1 {
		return nil
	}

	return sessions[index]
}

// Sessions returns the active sessions.
func (c Client) Sessions() ([]*Session, error) {
	var sessions []*Session

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

		sessions = append(sessions, session)
	}

	return sessions, err
}

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
		logger:            c.logger,
	}, nil
}

// Windows returns the windows of the given session.
func (c Client) Windows(session *Session) ([]*Window, error) {
	var windows []*Window

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

		windows = append(windows, window)
	}

	return windows, nil
}

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
	var panes []*Pane

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

	for line := range strings.SplitSeq(output, "\n") {
		pane, err := c.NewPaneFromLine(line, window)
		if err != nil {
			return panes, err
		}

		panes = append(panes, pane)
	}

	return panes, nil
}

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

// NewSession creates a new session with the given name and starting directory.
func (c Client) NewSession(sessionName, startingDirectory string) (*Session, error) {
	var session *Session

	sessionName = SanitizeSessionName(sessionName)

	args := []string{
		"new",
		"-d",
		"-s",
		escapeFormat(sessionName),
		"-c",
		escapeFormat(startingDirectory),
		"-F", formatActiveSessions,
		"-P",
	}

	cmd := newCommand(c, args...)

	c.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		var withOutput CommandErrorWithOutput
		if errors.As(err, &withOutput) && strings.Contains(withOutput.Output, "duplicate session") {
			return nil, fmt.Errorf("%w: %w", ErrDuplicateSession, err)
		}

		return session, err
	}

	session, err = c.NewSessionFromLine(output)
	if err != nil {
		return session, err
	}

	return session, nil
}

// KillSessionByName kills the given session by the specified session name.
// Performs an attempt at an exact match by prepending the given sanitized
// session name with "=" otherwise tmux will attempt to match on prefix.
func (c Client) KillSessionByName(sessionName string) error {
	sessionName = SanitizeSessionName(sessionName)

	cmd := newCommand(c, "kill-session", "-t", fmt.Sprintf(`=%s`, sessionName))

	c.logger.Debug(cmd.String())

	if _, err := cmd.ExecWithOutput(); err != nil {
		return fmt.Errorf(`session "%s" could not be killed: %w`, sessionName, err)
	}

	return nil
}

// FindSessionByName returns the session with the given name if it exists.
func (c Client) FindSessionByName(sessionName string) (*Session, error) {
	sessionName = SanitizeSessionName(sessionName)

	sessions, _ := c.Sessions()

	found := findSessionByName(sessions, sessionName)

	if found != nil {
		return found, nil
	}

	return nil, fmt.Errorf(`session "%s" not found`, sessionName)
}

// HasSession returns true if a session with exactly the given name exists; "=" stops tmux matching a prefix.
// An error means that glaze cannot reach the server, so it cannot know whether the session exists.
func (c Client) HasSession(sessionName string) (bool, error) {
	cmd := newCommand(c, "has-session", "-t", fmt.Sprintf(`=%s`, SanitizeSessionName(sessionName)))

	c.logger.Debug(cmd.String())

	if _, err := cmd.ExecWithOutput(); err != nil {
		return false, lookupFailure(err)
	}

	return true, nil
}

// AttachCommand returns the shell command that attaches a terminal to the session on this server.
func (c Client) AttachCommand(session *Session) string {
	args := []string{"tmux"}
	if c.socketName != "" {
		args = append(args, "-L", posixQuote(c.socketName))
	} else if c.socketPath != "" {
		args = append(args, "-S", posixQuote(c.socketPath))
	}

	return strings.Join(append(args, "attach", "-t", posixQuote("="+session.Name)), " ")
}

// SocketPath returns the path of the socket that the server listens on.
func (c Client) SocketPath() (string, error) {
	cmd := newCommand(c, "display-message", "-p", "#{socket_path}")

	c.logger.Debug(cmd.String())

	return cmd.ExecWithOutput()
}

// InsideServer reports whether glaze runs in a client of this server.
// tmux puts the socket path of the server at the start of $TMUX, so a different path means a different server.
func (c Client) InsideServer() (bool, error) {
	socket, _, _ := strings.Cut(os.Getenv("TMUX"), ",")
	if socket == "" {
		return false, nil
	}

	ours, err := c.SocketPath()
	if err != nil {
		// When no server runs on this socket, glaze cannot run inside it.
		return false, lookupFailure(err)
	}

	return filepath.Clean(socket) == filepath.Clean(ours), nil
}

// CurrentPane returns the pane that glaze runs in, or "" when glaze does not run in a pane of this server.
func (c Client) CurrentPane() (string, error) {
	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return "", nil
	}

	inside, err := c.InsideServer()
	if err != nil || !inside {
		return "", err
	}

	return pane, nil
}

// CurrentSession returns the session of the pane that glaze runs in, or nil when glaze does not run in a pane of this server.
func (c Client) CurrentSession() (*Session, error) {
	pane, err := c.CurrentPane()
	if err != nil || pane == "" {
		return nil, err
	}

	cmd := newCommand(c, "display-message", "-p", "-t", pane, formatActiveSessions)

	c.logger.Debug(cmd.String())

	output, err := cmd.ExecWithOutput()
	if err != nil {
		return nil, fmt.Errorf("could not determine current session: %w", err)
	}

	return c.NewSessionFromLine(output)
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
