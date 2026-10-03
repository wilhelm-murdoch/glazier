package cases

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// lsUnreachableArgs are the glaze arguments of each unreachable case. down and up also get the profile.
var lsUnreachableArgs = map[string][]string{
	"ls":   {"ls"},
	"down": {"down", "--session", "unr"},
	"up":   {"up", "--detached"},
}

// ls lists the sessions of a server; ls, down and up exit 4 when tmux is unreachable.
func TestLs(t *testing.T) {
	harness.Run(t, "ls_no_server", func(c *harness.Case) {
		r := c.Glaze("ls", "--socket-name", c.Socket)
		c.OK(r, "ls with no server")
		c.Equal("ls with no server prints nothing", "", r.Stdout)
		c.Match("ls with no server says so on stderr", "no tmux server is running", r.Stderr)
	})

	// With exit-empty off, a server can run with no sessions.
	harness.Run(t, "empty_server", func(c *harness.Case) {
		c.TmuxConf("set -g exit-empty off\n")
		c.Tmux("start-server")
		r := c.Glaze("ls", "--socket-name", c.Socket)
		c.OK(r, "ls on a server with no sessions")
		c.Match("ls on a server with no sessions prints the header", "NAME +WINDOWS +PATH", r.Stdout)
		c.Simple("es")
		c.OK(c.Up(), "up on a server with no sessions")
		c.SessionExists("up creates the session", "es")
	})

	// A socket in a directory without permissions gives tmux "Permission denied", also for its owner.
	for _, sub := range []string{"ls", "down", "up"} {
		harness.Run(t, sub+"_unreachable", func(c *harness.Case) {
			c.Simple("unr")
			dir, socket := lsUnreachableServer(c)
			args := append(append([]string{}, lsUnreachableArgs[sub]...), "--socket-path", socket)
			if sub != "ls" {
				args = append(args, "--profile-path", ".glaze")
			}
			r := c.Glaze(args...)
			c.ExitCode(r, sub+" exits 4 when tmux is unreachable", 4)
			c.Match(sub+" names the cause", "Permission denied", r.Stderr)
			c.Chmod(dir, 0o700)
			has := c.Exec(harness.Opts{Quiet: true}, c.Env().Tmux, "-S", socket, "has-session", "-t", "=unr")
			c.True(sub+" leaves the session alone", has.Code == 0, "no session unr: %s", has.Stderr)
		})
	}

	harness.Run(t, "ls_sessions", func(c *harness.Case) {
		c.Mkdir("a", "b dir")
		c.Tmux("new-session", "-d", "-s", "alpha", "-c", c.Path("a"))
		c.Tmux("new-window", "-t", "=alpha:")
		c.Tmux("new-session", "-d", "-s", "beta two", "-c", c.Path("b dir"))
		r := c.Glaze("ls", "--socket-name", c.Socket)
		c.OK(r, "ls")
		c.Match("ls header", "NAME +WINDOWS +PATH", r.Stdout)
		c.Match("ls alpha row", "alpha +2 +"+regexp.QuoteMeta(c.Path("a")), r.Stdout)
		c.Match("ls spaced name row", "beta two +1 +"+regexp.QuoteMeta(c.Path("b dir")), r.Stdout)
		c.NoMatch("no current-session marker when detached", `\*`, r.Stdout)
		c.Golden("ls output", "ls/sessions.txt", r.Stdout)
	})

	// The socket of the case is short enough for every platform; a socket in the work directory is not.
	harness.Run(t, "ls_socket_path", func(c *harness.Case) {
		c.Tmux("new-session", "-d", "-s", "sp")
		socket := c.Tmux("display-message", "-p", "#{socket_path}")
		r := c.Glaze("ls", "--socket-path", socket)
		c.OK(r, "ls --socket-path")
		c.Match("ls via socket path", "sp", r.Stdout)
	})
}

// lsUnreachableServer starts a server with the session unr on a socket in a short directory of its
// own, then removes every permission of that directory. It returns the directory and the socket.
func lsUnreachableServer(c *harness.Case) (string, string) {
	dir := c.ShortDir()
	socket := filepath.Join(dir, "s")
	r := c.Exec(harness.Opts{Quiet: true}, c.Env().Tmux, "-S", socket, "new-session", "-d", "-s", "unr")
	if r.Code != 0 {
		c.T().Fatalf("start the server on the socket: %s", r.Stderr)
	}
	c.Chmod(dir, 0)
	return dir, socket
}
