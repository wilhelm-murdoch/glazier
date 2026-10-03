package cases

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// logLevels are the values that --log-level accepts.
var logLevels = []string{"trace", "debug", "info", "warning", "error", "critical"}

// TestCLI checks the flags, the subcommands and the output streams of glaze.
func TestCLI(t *testing.T) {
	// The matrix tests a release and gives its version. A local build has no release version, so the case checks only the form.
	harness.Run(t, "cli_version", func(c *harness.Case) {
		r := c.Glaze("--version")
		c.OK(r, "--version")
		if v := c.Env().ExpectVersion; v != "" {
			c.Match("--version reports the expected version", "Version: "+regexp.QuoteMeta(v)+",", r.Stdout)
		} else {
			c.Match("--version reports a version, a commit and a date", `^Version: [^,]+, Stage: [^,]+, Commit: [^,]+, Date: .+$`, strings.TrimSpace(r.Stdout))
		}
	})

	// A profile is in place, so a -v that up ignores builds a session and NoServer fails.
	harness.Run(t, "cli_flag_after_subcommand", func(c *harness.Case) {
		c.Simple("flag-after-subcommand")
		c.ExitCode(upAfter(c, nil, "-v"), "-v after a subcommand is a usage error (exit 2)", exitUsage)
		c.NoServer("-v after a subcommand")
	})

	harness.Run(t, "cli_help", func(c *harness.Case) {
		r := c.Glaze("--help")
		c.OK(r, "--help")
		c.Match("--help shows the author as a name and an address", `Wilhelm Murdoch <wilhelm@devilmayco\.de>`, r.Stdout)
		for _, sub := range []string{"up", "down", "ls", "format", "save"} {
			c.OK(c.Glaze(sub, "--help"), fmt.Sprintf("%s --help", sub))
		}

		c.Fails(c.Glaze("bogus"), "unknown subcommand")
		c.Fails(c.Glaze("up", "--no-such-flag"), "unknown flag")
	})

	harness.Run(t, "cli_log_level", func(c *harness.Case) {
		c.Simple("log-level")
		c.Fails(upAfter(c, []string{"--log-level", "nope"}), "invalid --log-level")
		for _, level := range logLevels {
			c.OK(upAfter(c, []string{"--log-level", level}, "--clear"), fmt.Sprintf("--log-level %s accepted", level))
		}

		c.KillServer()
		r := upAfter(c, []string{"--log-level", "error"})
		c.NoMatch("--log-level error hides INF lines", "INF", r.Output())
	})

	harness.Run(t, "cli_output", func(c *harness.Case) {
		c.Simple("output")
		r := c.Up()
		c.Equal("up writes nothing to stdout", "", r.Stdout)
		c.Match("up writes its log to stderr", "INF", r.Stderr)
		c.NoMatch("no ANSI escapes when output is not a TTY", `\x1b\[`, r.Output())
	})

	// The pattern accepts new-session and its short name new, so it does not depend on the name that glaze uses.
	harness.Run(t, "cli_debug", func(c *harness.Case) {
		c.Simple("debug")
		r := c.Up("--debug")
		c.OK(r, "up --debug")
		c.Match("--debug prints tmux commands", `DBG .*tmux .* (new|new-session) -d`, r.Stderr)
	})
}

// upAfter runs "glaze <globals> up --detached" with args on the server of the
// case. A global flag such as --log-level must come before the subcommand.
func upAfter(c *harness.Case, globals []string, args ...string) *harness.Result {
	c.T().Helper()
	return c.Glaze(slices.Concat(globals, []string{"up", "--detached", "--socket-name", c.Socket}, args)...)
}
