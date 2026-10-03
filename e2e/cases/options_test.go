package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

func TestOptionsHooks(t *testing.T) {
	harness.Run(t, "options_all_scopes", func(c *harness.Case) {
		c.Fixture("options/all-scopes.glaze")
		c.OK(c.Up(), "up")
		c.Equal("session option history-limit", "4242", c.Tmux("show-options", "-t", "=op:", "-v", "history-limit"))
		c.Equal("session option status", "off", c.Tmux("show-options", "-t", "=op:", "-v", "status"))
		c.Equal("window option automatic-rename", "off", c.Tmux("show-options", "-w", "-t", "=op:w", "-v", "automatic-rename"))
		c.Equal("window option monitor-activity", "on", c.Tmux("show-options", "-w", "-t", "=op:w", "-v", "monitor-activity"))
		c.Equal("pane option remain-on-exit", "on", c.Tmux("show-options", "-p", "-t", "=op:w", "-v", "remain-on-exit"))
		c.Equal("window name survives automatic-rename", "w", c.WindowNames("op"))
	})

	// A window option declared on the session applies to every window, not only to the first.
	harness.Run(t, "options_session_declares_window_option", func(c *harness.Case) {
		c.Fixture("options/session-declares-window-option.glaze")
		c.OK(c.Up(), "up")
		c.Equal("window one remain-on-exit", "on", c.Tmux("show-options", "-w", "-t", c.WindowID("ow", "one"), "-v", "remain-on-exit"))
		c.Equal("window two remain-on-exit", "on", c.Tmux("show-options", "-w", "-t", c.WindowID("ow", "two"), "-v", "remain-on-exit"))
		c.Equal("session option history-limit", "4242", c.Tmux("show-options", "-t", "=ow:", "-v", "history-limit"))
	})

	// A session option declared on a window applies to the session, with a warning.
	harness.Run(t, "options_window_declares_session_option", func(c *harness.Case) {
		c.Fixture("options/window-declares-session-option.glaze")
		r := c.Up()
		c.OK(r, "up")
		c.Equal("session option history-limit", "4321", c.Tmux("show-options", "-t", "=os:", "-v", "history-limit"))
		c.Match("warns that the option applies to the session", `applies to the whole session`, r.Output())
	})

	harness.Run(t, "options_numeric_value", func(c *harness.Case) {
		c.Fixture("options/numeric-value.glaze")
		c.OK(c.Up(), "numeric (unquoted) option value")
		c.Equal("numeric option value set", "5000", c.Tmux("show-options", "-t", "=on:", "-v", "history-limit"))
	})

	harness.Run(t, "options_invalid_name", func(c *harness.Case) {
		c.Fixture("options/invalid-name.glaze")
		r := c.Up()
		c.Fails(r, "unknown option name fails")
		c.SessionGone("unknown option removes the partly built session", "oi")
		c.Match("unknown option error names the option", `invalid option: no-such-option`, r.Stderr)
	})

	harness.Run(t, "options_invalid_value", func(c *harness.Case) {
		c.Fixture("options/invalid-value.glaze")
		r := c.Up()
		c.Fails(r, "invalid option value fails")
		c.SessionGone("invalid option value removes the partly built session", "ov")
		c.Match("invalid option value error names the value", `bad value: maybe`, r.Stderr)
	})

	harness.Run(t, "options_user_option", func(c *harness.Case) {
		c.Fixture("options/user-option.glaze")
		c.OK(c.Up(), "user @option")
		c.Equal("user @option set", "yes", c.Tmux("show-options", "-t", "=ou:", "-v", "@my-flag"))
	})

	harness.Run(t, "hooks_all_scopes", func(c *harness.Case) {
		c.Fixture("hooks/all-scopes.glaze")
		c.OK(c.Up("--var", "dir="+c.Dir), "up")
		c.Match("session hook registered", `session-renamed`, c.Tmux("show-hooks", "-t", "=hk:"))
		c.Match("window hook registered", `window-renamed`, c.Tmux("show-hooks", "-w", "-t", "=hk:w"))
		c.Match("pane hook registered", `pane-focus-in`, c.Tmux("show-hooks", "-p", "-t", "=hk:w"))
		c.Tmux("rename-window", "-t", "=hk:w", "w2")
		c.Eventually("window hook fires", harness.Patience, func() (bool, string) { return c.Exists("h_window"), "no marker" })
		c.Tmux("rename-session", "-t", "=hk", "hk2")
		c.Eventually("session hook fires", harness.Patience, func() (bool, string) { return c.Exists("h_session"), "no marker" })
	})

	harness.Run(t, "hooks_invalid", func(c *harness.Case) {
		c.Fixture("hooks/invalid.glaze")
		r := c.Up()
		c.Fails(r, "unknown hook name fails")
		c.NoServer("unknown hook")
		c.Match("unknown hook diagnostic", `Invalid hook specified`, r.Stderr)
	})

	// The README example: a client-attached hook on the session, which runs when a client attaches.
	harness.Run(t, "hooks_readme_example", func(c *harness.Case) {
		c.Fixture("hooks/client-attached.glaze")
		c.OK(c.Up("--var", "dir="+c.Dir), "up")
		cc := c.AttachControl("hc")
		c.Eventually("client-attached hook (README example) fires", harness.Patience, func() (bool, string) {
			return c.Exists("h_attached"), "no file after a client attached"
		})
		cc.Close()
	})
}
