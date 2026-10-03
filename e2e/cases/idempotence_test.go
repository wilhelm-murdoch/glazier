package cases

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// The formats of idempotenceSnapshot: the full state, and the structure only.
const (
	idempotenceFullWindow = "W #{window_index}|#{window_name}|#{window_active}|#{window_panes}"
	idempotenceFullPane   = "  P #{pane_index}|#{pane_title}|#{pane_active}|#{pane_current_path}|#{pane_left},#{pane_top},#{pane_width}x#{pane_height}"
	idempotenceWindow     = "W #{window_index}|#{window_name}"
	idempotencePane       = "  P #{pane_index}|#{pane_title}"
)

// An up against a running session leaves it alone; --clear rebuilds it.
func TestIdempotence(t *testing.T) {
	harness.Run(t, "up_twice", func(c *harness.Case) {
		c.Fixture("idempotence/session-command.glaze")
		c.OK(c.Up(), "first up")
		c.WaitFile("o", harness.Patience)
		before := idempotenceSnapshot(c, "id", idempotenceFullWindow, idempotenceFullPane)
		r := c.Up()
		c.OK(r, "second up")
		c.Equal("second up leaves session untouched", before, idempotenceSnapshot(c, "id", idempotenceFullWindow, idempotenceFullPane))
		// A rerun would reach the pane before this line, so "end" marks the point to read the file.
		c.Tmux("send-keys", "-t", "=id:", "echo end >> o", "Enter")
		c.EventuallyEqual("session commands not rerun", "run,end", func() string { return c.Lines("o") })
		c.Logf("second up output: %q", r.Output())
	})

	harness.Run(t, "up_clear", func(c *harness.Case) {
		c.Fixture("idempotence/clear.glaze")
		c.OK(c.Up(), "first up")
		before := idempotenceSnapshot(c, "cl", idempotenceWindow, idempotencePane)
		c.Tmux("new-window", "-t", "=cl:", "-n", "extra")
		c.Tmux("split-window", "-t", "=cl:w1")
		c.OK(c.Up("--clear"), "up --clear")
		c.Equal("--clear rebuilds to profile", before, idempotenceSnapshot(c, "cl", idempotenceWindow, idempotencePane))
		c.Equal("--clear window list", "w1", c.WindowNames("cl"))
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
		// A race shows only on some runs, so the case tries four times.
		for range 4 {
			c.KillServer()
			args := []string{"up", "--detached", "--socket-name", c.Socket}
			p1 := c.Start(harness.Opts{}, c.Env().Glaze, args...)
			p2 := c.Start(harness.Opts{}, c.Env().Glaze, args...)
			r1, r2 := p1.Wait(), p2.Wait()
			codes = fmt.Sprintf("%d,%d", r1.Code, r2.Code)
			if codes != "0,0" {
				break
			}
		}
		c.Equal("two concurrent ups both succeed", "0,0", codes)
		c.Equal("two concurrent ups leave one session", "cc", strings.Join(c.Sessions(), ","))
		c.Equal("two concurrent ups build the session once", "w", c.WindowNames("cc"))
	})
}

// idempotenceSnapshot returns one line for each window and its panes, in the given formats.
func idempotenceSnapshot(c *harness.Case, session, windowFormat, paneFormat string) string {
	var b strings.Builder
	for _, w := range strings.Split(c.Tmux("list-windows", "-t", "="+session, "-F", "#{window_id}"), "\n") {
		if w == "" {
			continue
		}
		fmt.Fprintln(&b, c.Tmux("display-message", "-p", "-t", w, windowFormat))
		fmt.Fprintln(&b, c.Tmux("list-panes", "-t", w, "-F", paneFormat))
	}
	return b.String()
}
