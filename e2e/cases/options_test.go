package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// TestOptions checks that up sets each option at the scope where tmux keeps it.
func TestOptions(t *testing.T) {
	harness.Run(t, "options_all_scopes", func(c *harness.Case) {
		c.Fixture("options/all-scopes.glaze")
		c.OK(c.Up(), "up")
		c.Equal("session option history-limit", "4242", c.Option("=option-scopes:", "history-limit"))
		c.Equal("session option status", "off", c.Option("=option-scopes:", "status"))
		c.Equal("window option automatic-rename", "off", c.Option("=option-scopes:w", "automatic-rename", "-w"))
		c.Equal("window option monitor-activity", "on", c.Option("=option-scopes:w", "monitor-activity", "-w"))
		c.Equal("pane option remain-on-exit", "on", c.Option("=option-scopes:w", "remain-on-exit", "-p"))
		c.Equal("window name survives automatic-rename", "w", c.WindowNames("option-scopes"))
	})

	// A window option declared on the session applies to every window, not only to the first.
	harness.Run(t, "options_session_declares_window_option", func(c *harness.Case) {
		c.Fixture("options/session-declares-window-option.glaze")
		c.OK(c.Up(), "up")
		c.Equal("window one remain-on-exit", "on", c.Option(c.WindowID("session-declares-window-option", "one"), "remain-on-exit", "-w"))
		c.Equal("window two remain-on-exit", "on", c.Option(c.WindowID("session-declares-window-option", "two"), "remain-on-exit", "-w"))
		c.Equal("session option history-limit", "4242", c.Option("=session-declares-window-option:", "history-limit"))
	})

	// A session option declared on a window applies to the session, with a warning.
	harness.Run(t, "options_window_declares_session_option", func(c *harness.Case) {
		c.Fixture("options/window-declares-session-option.glaze")
		r := c.Up()
		c.OK(r, "up")
		c.Equal("session option history-limit", "4321", c.Option("=window-declares-session-option:", "history-limit"))
		c.Match("warns that the option applies to the session", `applies to the whole session`, r.Stderr)
	})

	harness.Run(t, "options_numeric_value", func(c *harness.Case) {
		c.Fixture("options/numeric-value.glaze")
		c.OK(c.Up(), "numeric (unquoted) option value")
		c.Equal("numeric option value set", "5000", c.Option("=numeric-option:", "history-limit"))
	})

	harness.Run(t, "options_invalid_name", func(c *harness.Case) {
		c.Fixture("options/invalid-name.glaze")
		r := c.Up()
		c.Fails(r, "unknown option name fails")
		c.SessionGone("unknown option removes the partly built session", "invalid-option-name")
		c.Match("unknown option error names the option", `invalid option: no-such-option`, r.Stderr)
	})

	harness.Run(t, "options_invalid_value", func(c *harness.Case) {
		c.Fixture("options/invalid-value.glaze")
		r := c.Up()
		c.Fails(r, "invalid option value fails")
		c.SessionGone("invalid option value removes the partly built session", "invalid-option-value")
		c.Match("invalid option value error names the value", `bad value: maybe`, r.Stderr)
	})

	harness.Run(t, "options_user_option", func(c *harness.Case) {
		c.Fixture("options/user-option.glaze")
		c.OK(c.Up(), "user @option")
		c.Equal("user @option set", "yes", c.Option("=user-option:", "@my-flag"))
	})
}
