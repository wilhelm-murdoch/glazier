package cases

import (
	"strconv"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// How glaze renders the diagnostics of an invalid profile, with and without a terminal, and what they say.
func TestDiagnostics(t *testing.T) {
	harness.Run(t, "diag_up_invalid_layout", func(c *harness.Case) {
		c.Fixture("diagnostics/invalid-layout.glaze")
		r := c.Up()
		c.Fails(r, "invalid profile")
		c.NoMatch("up diagnostics show source snippet", "source code not available", r.Stderr)
		c.NoMatch("no ANSI escapes in piped diagnostics", `\x1b\[`, r.Output())
		c.NoMatch("diagnostics go to stderr", "Error", r.Stdout)
		c.Golden("up diagnostics", "diagnostics/up-invalid-layout.txt", r.Stderr)
	})

	// On a terminal, diagnostics are coloured unless NO_COLOR is set.
	harness.Run(t, "diag_color_terminal", func(c *harness.Case) {
		c.Fixture("diagnostics/invalid-layout.glaze")
		r := c.StartTerminal(harness.Opts{}, c.Env().Glaze, "format", "--validate").Wait()
		c.Match("a terminal gets colour", `\x1b\[`, r.Stdout)
		r = c.StartTerminal(harness.Opts{Env: []string{"NO_COLOR=1"}}, c.Env().Glaze, "format", "--validate").Wait()
		c.NoMatch("NO_COLOR turns colour off on a terminal", `\x1b\[`, r.Stdout)
	})

	harness.Run(t, "diag_syntax_snippet", func(c *harness.Case) {
		c.Fixture("malformed/syntax-attribute-after-block.glaze")
		r := c.Glaze("format", "--validate")
		c.Fails(r, "syntax error rejected")
		c.NoMatch("syntax error shows the source", "source code not available", r.Stderr)
		c.Match("syntax error quotes the line", `window \{ pane \{\} name = "x"`, r.Stderr)
	})

	// The case runs glaze from an empty directory, so glaze looks for the profile in GLAZE_PATH.
	harness.Run(t, "diag_glaze_path_file", func(c *harness.Case) {
		c.Mkdir("empty")
		c.Simple("gpf", "gp/.glaze")
		c.Cd("empty")
		r := c.GlazeWith(harness.Opts{Env: []string{"GLAZE_PATH=" + c.Path("gp/.glaze")}}, "format", "--validate")
		c.Fails(r, "GLAZE_PATH that names a file rejected")
		c.Match("GLAZE_PATH hint", "GLAZE_PATH must be a directory", r.Stderr)
		c.Mkdir("dotdir/.glaze")
		c.Cd("dotdir")
		r = c.Glaze("format", "--validate")
		c.Fails(r, ".glaze directory rejected")
		c.Match(".glaze directory hint", "is a directory, not a profile", r.Stderr)
	})

	harness.Run(t, "diag_down_required_var", func(c *harness.Case) {
		c.Fixture("validation/required-var.glaze")
		r := c.Down()
		c.Fails(r, "down without a required variable")
		c.Match("down names the required variable", "Required variable not set", r.Stderr)
		c.NoMatch("down does not say Unsupported attribute", "Unsupported attribute", r.Stderr)
	})

	harness.Run(t, "diag_locals_cycle", func(c *harness.Case) {
		c.Fixture("validation/locals-cycle.glaze")
		r := c.Glaze("format", "--validate")
		c.Fails(r, "locals cycle rejected")
		c.Match("locals cycle reported as a cycle", "Circular reference between locals", r.Stderr)
		c.Equal("locals cycle reported once", "1", strconv.Itoa(strings.Count(r.Stderr, "Circular reference between locals")))
		c.NoMatch("locals cycle has no Unsupported attribute", "Unsupported attribute", r.Stderr)
	})

	harness.Run(t, "diag_random_map", func(c *harness.Case) {
		c.Fixture("validation/random-map.glaze")
		r := c.Glaze("format", "--validate")
		c.Fails(r, "random() of a map rejected")
		c.Match("random() of a map says it needs a list", "random requires a non-empty list", r.Stderr)
		c.NoMatch("random() of a map shows no stack dump", "goroutine", r.Stderr)
	})
}
