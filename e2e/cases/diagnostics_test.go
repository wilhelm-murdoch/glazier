package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// How glaze renders the diagnostics of an invalid profile, with and without a terminal.
func TestDiagnostics(t *testing.T) {
	harness.Run(t, "up_diagnostics", func(c *harness.Case) {
		c.Fixture("diagnostics/invalid-layout.glaze")
		r := c.Up()
		c.Fails(r, "invalid profile")
		c.NoMatch("up diagnostics show source snippet", "source code not available", r.Output())
		c.NoMatch("no ANSI escapes in piped diagnostics", `\x1b\[`, r.Output())
		c.NoMatch("diagnostics go to stderr", "Error", r.Stdout)
		c.Golden("up diagnostics", "diagnostics/up-invalid-layout.txt", r.Stderr)
	})

	// On a terminal, diagnostics are coloured unless NO_COLOR is set.
	harness.Run(t, "color_terminal", func(c *harness.Case) {
		c.Fixture("diagnostics/invalid-layout.glaze")
		r := c.StartTerminal(harness.Opts{}, c.Env().Glaze, "format", "--validate").Wait()
		c.Match("a terminal gets colour", `\x1b\[`, r.Stdout)
		r = c.StartTerminal(harness.Opts{Env: []string{"NO_COLOR=1"}}, c.Env().Glaze, "format", "--validate").Wait()
		c.NoMatch("NO_COLOR turns colour off on a terminal", `\x1b\[`, r.Stdout)
	})
}
