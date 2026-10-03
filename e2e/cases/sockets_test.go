package cases

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

func TestSockets(t *testing.T) {
	harness.Run(t, "socket_path", func(c *harness.Case) {
		sock := filepath.Join(c.ShortDir(), "s.sock")
		c.Simple("spth")
		c.OK(c.Glaze("up", "--detached", "--socket-path", sock), "up --socket-path")
		c.True("session on socket path", c.Exec(harness.Opts{Quiet: true}, c.Env().Tmux, "-S", sock, "has-session", "-t", "=spth").Code == 0, "missing")
		c.Match("ls --socket-path", "spth", c.Glaze("ls", "--socket-path", sock).Stdout)
		c.OK(c.Glaze("save", "--session", "spth", "--stdout", "--socket-path", sock), "save --socket-path")
		c.OK(c.Glaze("down", "--socket-path", sock), "down --socket-path")
		c.True("down via socket path", c.Exec(harness.Opts{Quiet: true}, c.Env().Tmux, "-S", sock, "has-session", "-t", "=spth").Code != 0, "still there")
	})

	// glaze gives --socket-name to tmux when both flags are set.
	harness.Run(t, "socket_both", func(c *harness.Case) {
		sock := filepath.Join(c.ShortDir(), "s.sock")
		c.Simple("sb")
		c.OK(c.Glaze("up", "--detached", "--socket-path", sock, "--socket-name", c.Socket), "up with both --socket-path and --socket-name")
		c.Equal("--socket-name wins over --socket-path", "sb", strings.Join(c.Sessions(), ","))
		onPath := c.Exec(harness.Opts{Quiet: true}, c.Env().Tmux, "-S", sock, "list-sessions", "-F", "#S")
		c.Equal("nothing on the --socket-path server", "", strings.TrimSpace(onPath.Stdout))
	})

	harness.Run(t, "socket_default_untouched", func(c *harness.Case) {
		c.Simple("sd")
		c.Up()
		c.True("--socket-name does not touch default server", c.TmuxDefault("list-sessions").Code != 0, "default server has sessions")
	})
}
