package cases

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// paneFixtures holds a profile for each pane count, with the layout in var.layout.
var paneFixtures = map[int]string{2: "layouts/two-panes.glaze", 3: "layouts/three-panes.glaze", 4: "layouts/four-panes.glaze"}

func TestLayouts(t *testing.T) {
	for _, preset := range []string{"even-horizontal", "even-vertical", "main-horizontal", "main-vertical", "tiled"} {
		for _, n := range []int{3, 4} {
			harness.Run(t, fmt.Sprintf("layout_%s_%d", preset, n), func(c *harness.Case) {
				c.Fixture(paneFixtures[n])
				c.OK(c.Up("--var", "layout="+preset), fmt.Sprintf("up %s x%d", preset, n))
				c.Equal(fmt.Sprintf("%s x%d geometry matches tmux", preset, n), c.ReferenceGeometry(preset, n), c.Geometry("=lay:w"))
				c.Equal(fmt.Sprintf("%s x%d pane order", preset, n), paneNames(n), c.PaneTitles("=lay:w"))
			})
		}
	}

	harness.Run(t, "layout_default_tiled", func(c *harness.Case) {
		c.Fixture("layouts/no-layout.glaze")
		c.OK(c.Up(), "up")
		c.Equal("omitted layout is tiled", c.ReferenceGeometry("tiled", 4), c.Geometry("=lay:w"))
	})

	harness.Run(t, "layout_raw_replay", func(c *harness.Case) {
		c.Tmux("new-session", "-d", "-s", "ref")
		c.Tmux("split-window", "-h", "-l", "20", "-t", "=ref:")
		c.Tmux("split-window", "-v", "-l", "5", "-t", "=ref:")
		raw, want := c.Tmux("display-message", "-p", "-t", "=ref:", "#{window_layout}"), c.Geometry("=ref:")
		c.Tmux("kill-session", "-t", "=ref")
		c.Fixture("layouts/three-panes.glaze")
		c.OK(c.Up("--var", "layout="+raw), "up with a raw layout")
		c.Equal("raw layout geometry replayed", want, c.Geometry("=lay:w"))
	})

	harness.Run(t, "layout_raw_bad_checksum", func(c *harness.Case) {
		raw := rawLayout(c, 2)
		bad := "0000" + raw[4:]
		if strings.HasPrefix(raw, "0000") {
			bad = "ffff" + raw[4:]
		}
		c.Fixture("layouts/two-panes.glaze")
		r := c.Up("--var", "layout="+bad)
		c.Fails(r, "raw layout with bad checksum fails at up")
		c.ExitCode(r, "a bad checksum is a failure of up (exit 1)", 1)
		c.SessionGone("a bad checksum leaves no session", "lay")
	})

	harness.Run(t, "layout_raw_wrong_pane_count", func(c *harness.Case) {
		raw := rawLayout(c, 3)
		c.Fixture("layouts/two-panes.glaze")
		r := c.Up("--var", "layout="+raw)
		c.ExitCode(r, "a raw layout for 3 panes in a window of 2 is an invalid profile (exit 3)", 3)
		c.NoServer("wrong pane count")
	})

	harness.Run(t, "layout_invalid", func(c *harness.Case) {
		c.Fixture("layouts/two-panes.glaze")
		for _, layout := range []string{"foo", "Tiled", "tiled ", "rm -rf /", "bb62,80x24"} {
			c.Fails(c.Up("--var", "layout="+layout), fmt.Sprintf("invalid layout [%s] rejected", layout))
			c.NoServer(fmt.Sprintf("invalid layout [%s]", layout))
		}
	})
}

// rawLayout returns the layout string of a window with n side-by-side panes and stops the server.
func rawLayout(c *harness.Case, n int) string {
	c.Tmux("new-session", "-d", "-s", "ref")
	for i := 1; i < n; i++ {
		c.Tmux("split-window", "-h", "-t", "=ref:")
	}
	raw := c.Tmux("display-message", "-p", "-t", "=ref:", "#{window_layout}")
	c.KillServer()
	return raw
}

// paneNames returns "p0,p1,...", the names of the panes in the layout fixtures.
func paneNames(n int) string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("p%d", i)
	}
	return strings.Join(names, ",")
}
