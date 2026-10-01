package tmux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsRunning reports whether a tmux server runs on the socket; `server-info` would need an attached client, so it uses `list-sessions`.
// An error means that glaze cannot reach the server, which is not the same as no server.
func (c Client) IsRunning() (bool, error) {
	if _, err := c.output("list-sessions"); err != nil {
		return false, lookupFailure(err)
	}

	return true, nil
}

// HasSession returns true if a session with exactly the given name exists; "=" stops tmux matching a prefix.
// An error means that glaze cannot reach the server, so it cannot know whether the session exists.
func (c Client) HasSession(sessionName string) (bool, error) {
	if _, err := c.output("has-session", "-t", "="+SanitizeSessionName(sessionName)); err != nil {
		return false, lookupFailure(err)
	}

	return true, nil
}

// SocketPath returns the path of the socket that the server listens on.
func (c Client) SocketPath() (string, error) {
	return c.output("display-message", "-p", "#{socket_path}")
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

	output, err := c.output("display-message", "-p", "-t", pane, formatActiveSessions)
	if err != nil {
		return nil, fmt.Errorf("could not determine current session: %w", err)
	}

	return c.NewSessionFromLine(output)
}

// Attach attaches the terminal to the session, or switches the current client when glaze runs inside this server.
// It returns ErrOtherServer when glaze runs inside another tmux server, because attaching there would nest a client.
func (c Client) Attach(session *Session) error {
	inside, err := c.InsideServer()
	if err != nil {
		return err
	}

	args := []string{"attach", "-t", session.Target()}
	switch {
	case inside:
		args = []string{"switchc", "-t", session.Target()}
	case os.Getenv("TMUX") != "":
		return ErrOtherServer
	}

	if err := c.run(args...); err != nil {
		return fmt.Errorf("could not attach to session `%s`: %w", session.Name, err)
	}

	return nil
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
