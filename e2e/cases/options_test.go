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

	// glaze sets synchronize-panes after it runs the commands, so each pane gets its own commands.
	harness.Run(t, "options_synchronize_panes_window", func(c *harness.Case) {
		c.Fixture("options/synchronize-panes-window.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("the first pane ran only its own command", "p", fileLines(c, "p"))
		c.EventuallyEqual("the second pane ran only its own command", "q", fileLines(c, "q"))
		c.Equal("the window is synchronized", "on", c.Option("=synchronize-panes-window:w", "synchronize-panes", "-w"))
		c.EventuallyEqual("no glaze buffer is left", "", pasteBuffers(c))
	})

	// The option from the session block waits for the session commands too, and a window can turn it off again.
	harness.Run(t, "options_synchronize_panes_session", func(c *harness.Case) {
		c.Fixture("options/synchronize-panes-session.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("pane p ran only its own command", "p", fileLines(c, "p"))
		c.EventuallyEqual("pane q ran only its own command", "q", fileLines(c, "q"))
		c.EventuallyEqual("pane r ran only its own command", "r", fileLines(c, "r"))
		c.EventuallyEqual("the session command ran once", "s", fileLines(c, "s"))
		c.Equal("window one is synchronized", "on", c.Option(c.WindowID("synchronize-panes-session", "one"), "synchronize-panes", "-w"))
		c.Equal("window two is not synchronized", "off", c.Option(c.WindowID("synchronize-panes-session", "two"), "synchronize-panes", "-w"))
		c.EventuallyEqual("no glaze buffer is left", "", pasteBuffers(c))
	})

	harness.Run(t, "options_user_option", func(c *harness.Case) {
		c.Fixture("options/user-option.glaze")
		c.OK(c.Up(), "user @option")
		c.Equal("user @option set", "yes", c.Option("=user-option:", "@my-flag"))
	})
}
