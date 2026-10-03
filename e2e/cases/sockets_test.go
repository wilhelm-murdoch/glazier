package cases

import (
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// glaze reaches the server that --socket-path or --socket-name names, and no other.
func TestSockets(t *testing.T) {
	harness.Run(t, "socket_path", func(c *harness.Case) {
		socket := c.ShortSocket()
		c.Simple("spth")
		c.OK(c.Glaze("up", "--detached", "--socket-path", socket), "up --socket-path")
		probe := c.TmuxAt(socket, "has-session", "-t", "=spth")
		c.True("session on socket path", probe.Succeeded(), "no session spth on the socket: %s", probe.Describe())
		c.Match("ls --socket-path", "spth", c.Glaze("ls", "--socket-path", socket).Stdout)
		c.OK(c.Glaze("save", "--session", "spth", "--stdout", "--socket-path", socket), "save --socket-path")
		c.OK(c.Glaze("down", "--socket-path", socket), "down --socket-path")
		probe = c.TmuxAt(socket, "has-session", "-t", "=spth")
		c.True("down via socket path", probe.Failed(), "session spth is still on the socket: %s", probe.Describe())
	})

	// glaze gives --socket-name to tmux when both flags are set.
	harness.Run(t, "socket_both", func(c *harness.Case) {
		socket := c.ShortSocket()
		c.Simple("sb")
		c.OK(c.Up("--socket-path", socket), "up with both --socket-path and --socket-name")
		c.Equal("--socket-name wins over --socket-path", "sb", strings.Join(c.Sessions(), ","))
		onPath := c.TmuxAt(socket, "list-sessions", "-F", "#S")
		c.Equal("nothing on the --socket-path server", "", strings.TrimSpace(onPath.Stdout))
	})

	harness.Run(t, "socket_default_untouched", func(c *harness.Case) {
		c.Simple("sd")
		c.Must(c.Up(), "up")
		c.True("--socket-name does not touch default server", c.TmuxDefault("list-sessions").Failed(), "default server has sessions")
	})
}
