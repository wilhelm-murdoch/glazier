package cases

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

const (
	// structureWindow and structurePane are the formats of State for the structure only: names and order.
	structureWindow = "W #{window_index}|#{window_name}"
	structurePane   = "  P #{pane_index}|#{pane_title}"

	// concurrentAttempts is how often up_concurrent starts two ups, because a race shows only on some runs.
	concurrentAttempts = 4
)

// An up against a running session leaves it alone; --clear rebuilds it.
func TestIdempotence(t *testing.T) {
	harness.Run(t, "up_twice", func(c *harness.Case) {
		c.Fixture("idempotence/session-command.glaze")
		c.OK(c.Up(), "first up")
		c.WaitFile("o", harness.Patience)
		c.WaitForPanes("up-twice")
		before := c.State("up-twice", harness.WindowState, harness.PaneState)
		r := c.Up()
		c.OK(r, "second up")
		if !c.Equal("second up leaves session untouched", before, c.State("up-twice", harness.WindowState, harness.PaneState)) {
			c.Snapshot("up-twice")
		}

		// A rerun would reach the pane before this line, so "end" marks the point to read the file.
		c.Type("=up-twice:", "echo end >> o")
		c.EventuallyEqual("session commands not rerun", "run,end", fileLines(c, "o"))
		c.Logf("second up output: %q", r.Output())
	})

	harness.Run(t, "up_clear", func(c *harness.Case) {
		c.Fixture("idempotence/clear.glaze")
		c.OK(c.Up(), "first up")
		before := c.State("clear-rebuild", structureWindow, structurePane)
		c.Tmux("new-window", "-t", "=clear-rebuild:", "-n", "extra")
		c.Tmux("split-window", "-t", "=clear-rebuild:w1")
		c.OK(c.Up("--clear"), "up --clear")
		c.Equal("--clear rebuilds to profile", before, c.State("clear-rebuild", structureWindow, structurePane))
		c.Equal("--clear window list", "w1", c.WindowNames("clear-rebuild"))
	})

	harness.Run(t, "up_clear_not_running", func(c *harness.Case) {
		c.Simple("cn")
		c.OK(c.Up("--clear"), "--clear with no server")
		c.SessionExists("created", "cn")
	})

	harness.Run(t, "up_clear_other_sessions", func(c *harness.Case) {
		c.Simple("cm")
		c.Tmux("new-session", "-d", "-s", "bystander")
		c.OK(c.Up("--clear"), "up --clear")
		c.SessionExists("bystander survives --clear", "bystander")
	})

	harness.Run(t, "up_existing_foreign", func(c *harness.Case) {
		c.Simple("fg")
		c.Tmux("new-session", "-d", "-s", "fg", "-n", "mine")
		c.OK(c.Up(), "up against a foreign session")
		c.Equal("foreign session untouched", "mine", c.WindowNames("fg"))
	})

	harness.Run(t, "up_concurrent", func(c *harness.Case) {
		c.Simple("cc")
		var codes string
		for range concurrentAttempts {
			c.KillServer()
			first, second := c.StartUp(harness.Opts{}), c.StartUp(harness.Opts{})
			codes = fmt.Sprintf("%d,%d", first.Wait().Code, second.Wait().Code)
			if codes != "0,0" {
				break
			}
		}

		c.Equal("two concurrent ups both succeed", "0,0", codes)
		c.Equal("two concurrent ups leave one session", "cc", strings.Join(c.Sessions(), ","))
		c.Equal("two concurrent ups build the session once", "w", c.WindowNames("cc"))
	})
}
