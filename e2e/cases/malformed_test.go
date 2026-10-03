package cases

import (
	"fmt"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// malformedProfiles are the profiles in fixtures/malformed/ that up must reject before tmux starts.
// The other files there are not canonical, for the cases of format and diagnostics.
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

// up rejects a profile with a wrong structure before it starts tmux.
func TestMalformed(t *testing.T) {
	// No profile here starts a server, so all of them share one case.
	harness.Run(t, "malformed", func(c *harness.Case) {
		for _, name := range malformedProfiles {
			c.Fixture("malformed/" + name)
			label := fmt.Sprintf("malformed: %s", name)
			c.Fails(c.Up(), label)
			c.NoServer(label)
		}
	})
}
