package harness

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// These helpers read the state back from tmux with plain tmux commands. They
// never go through glaze, so a bug in glaze cannot hide itself.

const tmuxTimeout = 10 * time.Second

var referenceCounter atomic.Int64

// Tmux runs tmux on the server of the case and returns its standard output
// without the final newlines. It ignores errors, like 2>/dev/null in a shell.
func (c *Case) Tmux(args ...string) string {
	c.t.Helper()
	return strings.TrimRight(c.TmuxResult(args...).Stdout, "\n")
}

// TmuxResult runs tmux on the server of the case and returns the full result.
func (c *Case) TmuxResult(args ...string) *Result {
	c.t.Helper()
	return c.TmuxOn(c.Socket, args...)
}

// TmuxOn runs tmux on another server, named with -L. The case stops every
// server in its TMUX_TMPDIR at the end.
func (c *Case) TmuxOn(socket string, args ...string) *Result {
	c.t.Helper()
	if socket != "" {
		args = append([]string{"-L", socket}, args...)
	}
	return c.Exec(Opts{Timeout: tmuxTimeout, Quiet: true}, c.env.Tmux, args...)
}

// TmuxDefault runs tmux without -L, on the default server of the case.
func (c *Case) TmuxDefault(args ...string) *Result {
	c.t.Helper()
	return c.TmuxOn("", args...)
}

// KillServer stops the server of the case and waits until it is gone. A new
// server that starts while the old one stops can lose its sessions.
func (c *Case) KillServer() {
	c.t.Helper()
	c.Tmux("kill-server")
	if !c.WaitUntil(Patience, func() bool { return !c.ServerRunning() }) {
		c.t.Errorf("the tmux server did not stop")
	}
}

// HasSession reports whether a session with exactly this name exists.
func (c *Case) HasSession(name string) bool {
	c.t.Helper()
	return c.TmuxResult("has-session", "-t", "="+name).Code == 0
}

// ServerRunning reports whether a tmux server answers on the socket of the case.
func (c *Case) ServerRunning() bool {
	c.t.Helper()
	return c.TmuxResult("list-sessions").Code == 0
}

// Sessions returns the names of all sessions.
func (c *Case) Sessions() []string {
	c.t.Helper()
	return c.lines("list-sessions", "-F", "#{session_name}")
}

// WindowNames returns the window names of a session in index order, joined with commas.
func (c *Case) WindowNames(session string) string {
	c.t.Helper()
	return strings.Join(c.lines("list-windows", "-t", "="+session, "-F", "#{window_name}"), ",")
}

// WindowCount returns the number of windows of a session.
func (c *Case) WindowCount(session string) int {
	c.t.Helper()
	return len(c.lines("list-windows", "-t", "="+session, "-F", "#{window_id}"))
}

// WindowID returns the id (@N) of the first window of a session with this name.
func (c *Case) WindowID(session, name string) string {
	c.t.Helper()
	for _, line := range c.lines("list-windows", "-t", "="+session, "-F", "#{window_id} #{window_name}") {
		if id, n, _ := strings.Cut(line, " "); n == name {
			return id
		}
	}
	return ""
}

// ActiveWindow returns the name of the active window of a session.
func (c *Case) ActiveWindow(session string) string {
	c.t.Helper()
	return c.active("list-windows", "-t", "="+session, "-F", "#{window_active} #{window_name}")
}

// PaneTitles returns the pane titles of a window target in index order, joined with commas.
func (c *Case) PaneTitles(target string) string {
	c.t.Helper()
	return strings.Join(c.lines("list-panes", "-t", target, "-F", "#{pane_title}"), ",")
}

// PanePaths returns the current paths of the panes of a window target, joined with commas.
func (c *Case) PanePaths(target string) string {
	c.t.Helper()
	return strings.Join(c.lines("list-panes", "-t", target, "-F", "#{pane_current_path}"), ",")
}

// ActivePane returns the title of the active pane of a window target.
func (c *Case) ActivePane(target string) string {
	c.t.Helper()
	return c.active("list-panes", "-t", target, "-F", "#{pane_active} #{pane_title}")
}

// PaneSize returns "WIDTHxHEIGHT" of the pane with this title in a window target.
func (c *Case) PaneSize(target, title string) string {
	c.t.Helper()
	for _, line := range c.lines("list-panes", "-t", target, "-F", "#{pane_width}x#{pane_height} #{pane_title}") {
		if size, t, _ := strings.Cut(line, " "); t == title {
			return size
		}
	}
	return ""
}

// Geometry returns "LEFT,TOP,WIDTHxHEIGHT" for each pane of a window target, joined with spaces.
func (c *Case) Geometry(target string) string {
	c.t.Helper()
	return strings.Join(c.lines("list-panes", "-t", target, "-F", "#{pane_left},#{pane_top},#{pane_width}x#{pane_height}"), " ")
}

// ReferenceGeometry builds a window with n panes on the server of the case
// and returns the geometry that tmux itself gives to a layout.
func (c *Case) ReferenceGeometry(layout string, panes int) string {
	c.t.Helper()
	s := fmt.Sprintf("ref%d", referenceCounter.Add(1))
	c.Tmux("new-session", "-d", "-s", s)
	for i := 1; i < panes; i++ {
		c.Tmux("split-window", "-t", "="+s+":")
		c.Tmux("select-layout", "-t", "="+s+":", "tiled")
	}
	c.Tmux("select-layout", "-t", "="+s+":", layout)
	g := c.Geometry("=" + s + ":")
	c.Tmux("kill-session", "-t", "="+s)
	return g
}

// Type types a line into a pane of the server of the case and presses Enter,
// as a user does. Quote each word with ShellQuote.
func (c *Case) Type(target, line string) {
	c.t.Helper()
	c.TypeOn(c.Socket, target, line)
}

// TypeOn types a line into a pane of another server of the case.
func (c *Case) TypeOn(socket, target, line string) {
	c.t.Helper()
	c.TmuxOn(socket, "send-keys", "-t", target, "-l", line)
	c.TmuxOn(socket, "send-keys", "-t", target, "Enter")
}

// ShellQuote quotes a word for a POSIX shell.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ClientSessions returns the session of each attached client.
func (c *Case) ClientSessions() []string {
	c.t.Helper()
	return c.lines("list-clients", "-F", "#{client_session}")
}

// Snapshot logs every window and pane of a session, for the log of a failure.
func (c *Case) Snapshot(session string) {
	c.t.Helper()
	var b strings.Builder
	for _, w := range c.lines("list-windows", "-t", "="+session, "-F", "#{window_id}") {
		fmt.Fprintf(&b, "%s\n", c.Tmux("display-message", "-p", "-t", w, "W #{window_index}|#{window_name}|#{window_active}|#{window_layout}"))
		for _, p := range c.lines("list-panes", "-t", w, "-F", "  P #{pane_index}|#{pane_title}|#{pane_active}|#{pane_current_path}|#{pane_left},#{pane_top},#{pane_width}x#{pane_height}") {
			fmt.Fprintf(&b, "%s\n", p)
		}
	}
	c.t.Logf("snapshot of %s:\n%s", session, b.String())
}

// lines runs a tmux query and returns its lines. tmux 3.4, and only 3.4,
// prints a $ that starts a variable name as \$ in plain -F output.
func (c *Case) lines(args ...string) []string {
	c.t.Helper()
	out := c.Tmux(args...)
	if out == "" {
		return nil
	}
	if c.env.TmuxVersion == "3.4" {
		out = strings.ReplaceAll(out, `\$`, "$")
	}
	return strings.Split(out, "\n")
}

func (c *Case) active(args ...string) string {
	c.t.Helper()
	for _, line := range c.lines(args...) {
		if name, ok := strings.CutPrefix(line, "1 "); ok {
			return name
		}
	}
	return ""
}
