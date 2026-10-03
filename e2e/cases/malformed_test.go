package cases

import (
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// malformedLabelLength is the length of the start of a profile that a label shows.
const malformedLabelLength = 60

// malformedProfiles are broken profiles in fixtures/malformed/. up must reject each one before tmux starts.
var malformedProfiles = []string{
	"empty.glaze",
	"empty-session.glaze",
	"empty-window.glaze",
	"two-sessions.glaze",
	"labelled-session.glaze",
	"unknown-attribute.glaze",
	"focus-not-bool.glaze",
	"no-session.glaze",
	"name-is-list.glaze",
	"commands-not-list.glaze",
}

func TestMalformed(t *testing.T) {
	harness.Run(t, "diag_syntax_snippet", func(c *harness.Case) {
		c.Fixture("malformed/diag-syntax-error.glaze")
		r := c.Glaze("format", "--validate")
		c.Fails(r, "syntax error rejected")
		c.NoMatch("syntax error shows the source", "source code not available", r.Stderr)
		c.Match("syntax error quotes the line", `window \{ pane \{\} name = "x"`, r.Stderr)
	})

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
		c.NoMatch("locals cycle has no Unsupported attribute", "Unsupported attribute", r.Stderr)
	})

	harness.Run(t, "diag_random_map", func(c *harness.Case) {
		c.Fixture("validation/random-map.glaze")
		r := c.Glaze("format", "--validate")
		c.Fails(r, "random() of a map rejected")
		c.Match("random() of a map says it needs a list", "random requires a non-empty list", r.Stderr)
		c.NoMatch("random() of a map shows no stack dump", "goroutine", r.Stderr)
	})

	// No profile here starts a server, so all of them share one case.
	harness.Run(t, "malformed", func(c *harness.Case) {
		for _, name := range malformedProfiles {
			c.Fixture("malformed/" + name)
			label := "malformed: " + malformedLabel(c.Read(".glaze"))
			c.Fails(c.Up(), label)
			c.NoServer(label)
		}
	})
}

// malformedLabel shows the start of a profile on one line, so the label names the profile.
func malformedLabel(profile string) string {
	line := []byte(strings.ReplaceAll(profile, "\n", " "))
	if len(line) > malformedLabelLength {
		line = line[:malformedLabelLength]
	}
	return string(line)
}
