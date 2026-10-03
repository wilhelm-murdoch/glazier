package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// TestHooks checks that up registers each hook at its scope and that tmux runs it.
func TestHooks(t *testing.T) {
	harness.Run(t, "hooks_all_scopes", func(c *harness.Case) {
		c.Fixture("hooks/all-scopes.glaze")
		c.OK(c.Up(), "up")
		c.Match("session hook registered", `session-renamed`, c.Tmux("show-hooks", "-t", "=hook-scopes:"))
		c.Match("window hook registered", `window-renamed`, c.Tmux("show-hooks", "-w", "-t", "=hook-scopes:w"))
		c.Match("pane hook registered", `pane-focus-in`, c.Tmux("show-hooks", "-p", "-t", "=hook-scopes:w"))
		c.TmuxSetup("rename-window", "-t", "=hook-scopes:w", "w2")
		c.EventuallyExists("window hook fires", "h_window")
		c.TmuxSetup("rename-session", "-t", "=hook-scopes", "hook-scopes-renamed")
		c.EventuallyExists("session hook fires", "h_session")
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
		c.OK(c.Up(), "up")
		cc := c.AttachControl("client-attached")
		c.EventuallyExists("client-attached hook (README example) fires", "h_attached")
		cc.Close()
	})
}
