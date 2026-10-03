package cases

import (
	"regexp"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

func TestCLI(t *testing.T) {
	harness.Run(t, "cli_basics", func(c *harness.Case) {
		r := c.Glaze("--version")
		c.OK(r, "--version")
		// The matrix tests a release and gives its version. A local build has no release version, so only the form is checked.
		if v := c.Env().ExpectVersion; v != "" {
			c.Match("--version reports the expected version", "Version: "+regexp.QuoteMeta(v)+",", r.Stdout)
		} else {
			c.Match("--version reports a version, a commit and a date", `^Version: [^,]+, Stage: [^,]+, Commit: [^,]+, Date: .+$`, strings.TrimSpace(r.Stdout))
		}

		// With a profile in place, a -v that up ignores would build the session.
		c.Simple("vf")
		r = c.Glaze("up", "-v", "--detached", "--socket-name", c.Socket)
		c.ExitCode(r, "-v after a subcommand is a usage error (exit 2)", 2)
		c.NoServer("-v after a subcommand")
		c.Remove(".glaze")

		r = c.Glaze("--help")
		c.OK(r, "--help")
		c.Match("--help shows the author as a name and an address", `Wilhelm Murdoch <wilhelm@devilmayco\.de>`, r.Stdout)
		for _, sub := range []string{"up", "down", "ls", "format", "save"} {
			c.OK(c.Glaze(sub, "--help"), sub+" --help")
		}
		c.Fails(c.Glaze("bogus"), "unknown subcommand")
		c.Fails(c.Glaze("up", "--no-such-flag"), "unknown flag")

		c.Simple("lv")
		c.Fails(c.Glaze("--log-level", "nope", "up", "--detached", "--socket-name", c.Socket), "invalid --log-level")
		for _, level := range []string{"trace", "debug", "info", "warning", "error", "critical"} {
			c.OK(c.Glaze("--log-level", level, "up", "--detached", "--clear", "--socket-name", c.Socket), "--log-level "+level+" accepted")
		}
		c.KillServer()
		r = c.Glaze("--log-level", "error", "up", "--detached", "--socket-name", c.Socket)
		c.NoMatch("--log-level error hides INF lines", "INF", r.Output())

		c.KillServer()
		r = c.Up()
		c.Equal("up writes nothing to stdout", "", r.Stdout)
		c.Match("up writes its log to stderr", "INF", r.Stderr)
		c.NoMatch("no ANSI escapes when output is not a TTY", `\x1b\[`, r.Output())

		c.KillServer()
		r = c.Up("--debug")
		c.OK(r, "up --debug")
		c.Match("--debug prints tmux commands", "new-session|new|neww|splitw|split-window", r.Output())
	})
}
