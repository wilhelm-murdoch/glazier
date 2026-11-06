package cases

import (
	"fmt"
	"os"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// fakeTmuxScript answers -V with a fixed line and passes every other call to the real tmux.
const fakeTmuxScript = `#!/bin/sh
for arg in "$@"; do
	if [ "$arg" = "-V" ]; then
		echo "%s"
		exit 0
	fi
done
exec "%s" "$@"
`

// fakeTmux puts a tmux on PATH that reports version as its version, and returns the options that make glaze use it.
func fakeTmux(c *harness.Case, version string) harness.Opts {
	c.Write("bin/tmux", fmt.Sprintf(fakeTmuxScript, version, c.Env().Tmux))
	c.Chmod("bin/tmux", 0o755)

	return harness.Opts{Env: []string{"PATH=" + c.Path("bin") + string(os.PathListSeparator) + os.Getenv("PATH")}}
}

// TestVersion checks the tmux version floor: glaze needs tmux 3.2a or later, and a version it cannot read only warns.
func TestVersion(t *testing.T) {
	harness.Run(t, "version_old_tmux", func(c *harness.Case) {
		c.Simple("old-tmux")
		opts := fakeTmux(c, "tmux 3.0a")

		r := c.UpWith(opts)
		c.ExitCode(r, "up on tmux 3.0a exits 4", exitUnreachable)
		c.Match("up names the found and the needed version", `found tmux 3\.0a, glaze needs tmux 3\.2a or later`, r.Stderr)
		c.NoServer("up on tmux 3.0a starts no server")

		r = c.GlazeWith(opts, "save", "--socket-name", c.Socket, "--stdout")
		c.ExitCode(r, "save on tmux 3.0a exits 4", exitUnreachable)
		c.Match("save names the found version", `found tmux 3\.0a`, r.Stderr)
	})

	harness.Run(t, "version_unknown_tmux", func(c *harness.Case) {
		c.Simple("unknown-tmux")
		opts := fakeTmux(c, "tmux openbsd-7.6")

		r := c.UpWith(opts)
		c.OK(r, "up on a tmux with a version glaze cannot read")
		c.Match("up warns about the version", `could not read the tmux version`, r.Stderr)
		c.Match("the warning shows what tmux printed", `openbsd-7\.6`, r.Stderr)
		c.SessionExists("session on a tmux with an unknown version", "unknown-tmux")
	})

	harness.Run(t, "version_current_tmux", func(c *harness.Case) {
		c.Simple("current-tmux")
		r := c.Up()
		c.OK(r, "up on the tmux under test")
		c.NoMatch("up does not warn about the version", `tmux version`, r.Stderr)
		c.SessionExists("session on the tmux under test", "current-tmux")
	})
}
