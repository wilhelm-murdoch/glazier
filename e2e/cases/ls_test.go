package cases

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// unreachableCommands are the glaze commands that must exit 4 when tmux is unreachable.
var unreachableCommands = []struct {
	name string
	args []string
}{
	{"ls", []string{"ls"}},
	{"down", []string{"down", "--session", "unr", "--profile-path", ".glaze"}},
	{"up", []string{"up", "--detached", "--profile-path", ".glaze"}},
}

// ls lists the sessions of a server.
func TestLs(t *testing.T) {
	harness.Run(t, "ls_no_server", func(c *harness.Case) {
		r := c.Ls()
		c.OK(r, "ls with no server")
		c.Equal("ls with no server prints nothing", "", r.Stdout)
		c.Match("ls with no server says so on stderr", "no tmux server is running", r.Stderr)
	})

	// With exit-empty off, a server can run with no sessions.
	harness.Run(t, "ls_empty_server", func(c *harness.Case) {
		c.TmuxConf("set -g exit-empty off\n")
		c.Tmux("start-server")
		r := c.Ls()
		c.OK(r, "ls on a server with no sessions")
		c.Match("ls on a server with no sessions prints the header", "NAME +WINDOWS +PATH", r.Stdout)
		c.Simple("es")
		c.OK(c.Up(), "up on a server with no sessions")
		c.SessionExists("up creates the session", "es")
	})

	harness.Run(t, "ls_sessions", func(c *harness.Case) {
		c.Mkdir("a", "b dir")
		c.TmuxSetup("new-session", "-d", "-s", "alpha", "-c", c.Path("a"))
		c.TmuxSetup("new-window", "-t", "=alpha:")
		c.TmuxSetup("new-session", "-d", "-s", "beta two", "-c", c.Path("b dir"))
		r := c.Ls()
		c.OK(r, "ls")
		c.Match("ls header", "NAME +WINDOWS +PATH", r.Stdout)
		c.Match("ls alpha row", "alpha +2 +"+regexp.QuoteMeta(c.Path("a")), r.Stdout)
		c.Match("ls spaced name row", "beta two +1 +"+regexp.QuoteMeta(c.Path("b dir")), r.Stdout)
		c.NoMatch("no current-session marker when detached", `\*`, r.Stdout)
		c.Golden("ls output", "ls/sessions.txt", r.Stdout)
	})

	// The socket of the case is short enough for every platform; a socket in the work directory is not.
	harness.Run(t, "ls_socket_path", func(c *harness.Case) {
		c.TmuxSetup("new-session", "-d", "-s", "sp")
		socket := c.Tmux("display-message", "-p", "#{socket_path}")
		r := c.Glaze("ls", "--socket-path", socket)
		c.OK(r, "ls --socket-path")
		c.Match("ls via socket path", "sp", r.Stdout)
	})
}

// ls, down and up exit 4 when tmux is unreachable and leave the session alone.
func TestUnreachable(t *testing.T) {
	// A socket in a directory without permissions gives tmux "Permission denied", also for its owner.
	for _, command := range unreachableCommands {
		harness.Run(t, "unreachable_"+command.name, func(c *harness.Case) {
			c.Simple("unr")
			dir, socket := unreachableServer(c)
			r := c.Glaze(slices.Concat(command.args, []string{"--socket-path", socket})...)
			c.ExitCode(r, fmt.Sprintf("%s exits 4 when tmux is unreachable", command.name), exitUnreachable)
			c.Match(fmt.Sprintf("%s names the cause", command.name), "Permission denied", r.Stderr)
			// Give the permissions back, so the check can reach the server.
			c.Chmod(dir, 0o700)
			probe := c.TmuxAt(socket, "has-session", "-t", "=unr")
			c.True(fmt.Sprintf("%s leaves the session alone", command.name), probe.Succeeded(), "no session unr: %s", probe.Describe())
		})
	}
}

// unreachableServer starts a server with the session unr on a socket in a short directory of its
// own, then removes every permission of that directory. It returns the directory and the socket.
func unreachableServer(c *harness.Case) (dir, socket string) {
	dir = c.ShortDir()
	socket = filepath.Join(dir, "s")
	c.Must(c.TmuxAt(socket, "new-session", "-d", "-s", "unr"), "start the server on the socket")
	c.Chmod(dir, 0)
	return dir, socket
}
