package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

func TestEnvs(t *testing.T) {
	harness.Run(t, "envs_debug_redacted", func(c *harness.Case) {
		c.Fixture("envs/token.glaze")
		r := c.Up("--debug")
		c.OK(r, "up --debug with envs")
		c.NoMatch("--debug does not print an env value", `hunter2`, r.Output())
		c.Match("--debug shows the env key with the value redacted", `TOKEN <redacted>`, r.Output())
		c.Equal("the session still gets the env value", "TOKEN=hunter2", c.Tmux("show-environment", "-t", "=er", "TOKEN"))
	})

	harness.Run(t, "cmds_text_at_debug", func(c *harness.Case) {
		c.Fixture("envs/command-text.glaze")
		r := c.Up()
		c.OK(r, "up with a command")
		c.NoMatch("the default log level does not print command text", `hunter3`, r.Output())
		c.Match("the default log level counts the commands", `running pane commands.*count=1`, r.Output())
		c.KillServer()
		r = c.Up("--debug")
		c.OK(r, "up --debug with a command")
		c.Match("--debug prints command text", `hunter3`, r.Output())
	})

	harness.Run(t, "envs_session", func(c *harness.Case) {
		c.Fixture("envs/session.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("envs reach first window panes", `[bar baz][][it's "q"]`, func() string { return c.Lines("o1") })
		c.EventuallyEqual("envs reach later windows", "bar baz", func() string { return c.Lines("o2") })
		c.Equal("session environment has FOO", "FOO=bar baz", c.Tmux("show-environment", "-t", "=ev", "FOO"))
	})

	for _, scope := range []string{"window", "pane"} {
		harness.Run(t, "envs_on_"+scope, func(c *harness.Case) {
			c.Fixture("envs/on-" + scope + ".glaze")
			r := c.Up()
			c.Fails(r, "envs on "+scope+" rejected")
			c.NoServer("envs on " + scope)
			c.Match("envs on "+scope+" diagnostic points at session", `session`, r.Output())
			c.Match("envs on "+scope+" diagnostic says envs is not expected", `An argument named "envs" is not expected here`, r.Output())
		})
	}
}
