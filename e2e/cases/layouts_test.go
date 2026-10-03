package cases

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

var (
	// layoutFixtures holds a fixture for each pane count, with the layout in var.layout.
	layoutFixtures = map[int]layoutFixture{
		2: {"layouts/two-panes.glaze", "=two-panes:w"},
		3: {"layouts/three-panes.glaze", "=three-panes:w"},
		4: {"layouts/four-panes.glaze", "=four-panes:w"},
	}

	// layoutPresets are the named layouts of tmux.
	layoutPresets = []string{"even-horizontal", "even-vertical", "main-horizontal", "main-vertical", "tiled"}
)

// layoutFixture is a layouts fixture and its window target.
type layoutFixture struct{ path, target string }

// TestLayouts checks that a layout gives the same geometry as tmux gives, for a preset and for a raw layout string.
func TestLayouts(t *testing.T) {
	for _, preset := range layoutPresets {
		for _, n := range []int{3, 4} {
			harness.Run(t, fmt.Sprintf("layout_%s_%d", preset, n), func(c *harness.Case) {
				f := layoutFixtures[n]
				c.Fixture(f.path)
				c.OK(c.Up("--var", "layout="+preset), fmt.Sprintf("up %s x%d", preset, n))
				c.Equal(fmt.Sprintf("%s x%d geometry matches tmux", preset, n), c.ReferenceGeometry(preset, n), c.Geometry(f.target))
				c.Equal(fmt.Sprintf("%s x%d pane order", preset, n), paneNames(n), c.PaneTitles(f.target))
			})
		}
	}

	harness.Run(t, "layout_default_tiled", func(c *harness.Case) {
		c.Fixture("layouts/no-layout.glaze")
		c.OK(c.Up(), "up")
		c.Equal("omitted layout is tiled", c.ReferenceGeometry("tiled", 4), c.Geometry("=no-layout:w"))
	})

	// The reference window has panes of different sizes, so a preset cannot give the same geometry by chance.
	harness.Run(t, "layout_raw_replay", func(c *harness.Case) {
		c.TmuxSetup("new-session", "-d", "-s", "ref")
		c.TmuxSetup("split-window", "-h", "-l", "20", "-t", "=ref:")
		c.TmuxSetup("split-window", "-v", "-l", "5", "-t", "=ref:")
		raw := c.Tmux("display-message", "-p", "-t", "=ref:", "#{window_layout}")
		want := c.Geometry("=ref:")
		c.TmuxSetup("kill-session", "-t", "=ref")
		c.Fixture("layouts/three-panes.glaze")
		c.OK(c.Up("--var", "layout="+raw), "up with a raw layout")
		c.Equal("raw layout geometry replayed", want, c.Geometry("=three-panes:w"))
	})

	harness.Run(t, "layout_raw_bad_checksum", func(c *harness.Case) {
		raw := rawLayout(c, 2)
		if strings.HasPrefix(raw, "{") {
			c.T().Skip("tmux prints a JSON layout, which has no checksum")
		}

		bad := corruptChecksum(raw)
		c.Fixture("layouts/two-panes.glaze")
		r := c.Up("--var", "layout="+bad)
		c.Fails(r, "raw layout with bad checksum fails at up")
		c.ExitCode(r, "a bad checksum is a failure of up (exit 1)", exitFailure)
		c.SessionGone("a bad checksum leaves no session", "two-panes")
	})

	harness.Run(t, "layout_raw_wrong_pane_count", func(c *harness.Case) {
		raw := rawLayout(c, 3)
		c.Fixture("layouts/two-panes.glaze")
		r := c.Up("--var", "layout="+raw)
		c.ExitCode(r, "a raw layout for 3 panes in a window of 2 is an invalid profile (exit 3)", exitInvalidProfile)
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

// corruptChecksum changes the checksum of a raw layout, which is its first four hex digits.
func corruptChecksum(raw string) string {
	if strings.HasPrefix(raw, "0000") {
		return "ffff" + raw[4:]
	}

	return "0000" + raw[4:]
}

// paneNames returns "p0,p1,...", the names of the panes in the layouts fixtures.
func paneNames(n int) string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("p%d", i)
	}

	return strings.Join(names, ",")
}
