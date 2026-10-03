package cases

import (
	"fmt"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// TestEnvs checks that session envs reach every pane, stay out of the log and are valid on the session only.
func TestEnvs(t *testing.T) {
	harness.Run(t, "envs_debug_redacted", func(c *harness.Case) {
		c.Fixture("envs/token.glaze")
		r := c.Up("--debug")
		c.OK(r, "up --debug with envs")
		c.NoMatch("--debug does not print an env value", `hunter2`, r.Output())
		c.Match("--debug shows the env key with the value redacted", `TOKEN <redacted>`, r.Stderr)
		c.Equal("the session still gets the env value", "TOKEN=hunter2", c.Tmux("show-environment", "-t", "=token", "TOKEN"))
	})

	harness.Run(t, "envs_session", func(c *harness.Case) {
		c.Fixture("envs/session.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("envs reach first window panes", `[bar baz][][it's "q"]`, fileLines(c, "o1"))
		c.EventuallyEqual("envs reach later windows", "bar baz", fileLines(c, "o2"))
		c.Equal("session environment has FOO", "FOO=bar baz", c.Tmux("show-environment", "-t", "=session-envs", "FOO"))
	})

	for _, tc := range []struct{ scope, fixture string }{
		{"window", "envs/on-window.glaze"},
		{"pane", "envs/on-pane.glaze"},
	} {
		harness.Run(t, "envs_on_"+tc.scope, func(c *harness.Case) {
			c.Fixture(tc.fixture)
			r := c.Up()
			c.Fails(r, fmt.Sprintf("envs on %s rejected", tc.scope))
			c.NoServer(fmt.Sprintf("envs on %s", tc.scope))
			c.Match(fmt.Sprintf("envs on %s diagnostic points at session", tc.scope), `line \d+, in session:`, r.Stderr)
			c.Match(fmt.Sprintf("envs on %s diagnostic says envs is not expected", tc.scope), `An argument named "envs" is not expected here`, r.Stderr)
		})
	}
}
