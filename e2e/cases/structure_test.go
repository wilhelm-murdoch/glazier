package cases

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// TestStructure checks that up builds the windows and panes of a profile in file order.
func TestStructure(t *testing.T) {
	harness.Run(t, "struct_basic", func(c *harness.Case) {
		c.Fixture("structure/three-windows.glaze")
		c.OK(c.Up(), "up")
		w1 := c.WindowID("three-windows", "w1")
		passed := []bool{
			c.Equal("window order", "w1,w2,w3", c.WindowNames("three-windows")),
			c.Equal("window indexes start at base-index 0", "0,1,2", c.WindowIndexes("three-windows")),
			c.Equal("w1 pane order", "a,b,c", c.PaneTitles(w1)),
			c.Equal("w1 pane indexes", "0,1,2", c.PaneIndexes(w1)),
			c.Equal("w2 panes", "d", c.PaneTitles(c.WindowID("three-windows", "w2"))),
			c.Equal("w3 pane order", "e,f", c.PaneTitles(c.WindowID("three-windows", "w3"))),
			c.Equal("one session only", "three-windows", strings.Join(c.Sessions(), ",")),
		}

		if slices.Contains(passed, false) {
			c.Snapshot("three-windows")
		}
	})

	harness.Run(t, "struct_defaults", func(c *harness.Case) {
		c.Fixture("structure/no-names.glaze")
		c.OK(c.Up(), "up with no names")
		c.SessionExists("session name defaults to 'default'", "default")
		c.Equal("window name defaults to 'default'", "default", c.WindowNames("default"))
		c.Equal("pane names default to 'default'", "default,default", c.PaneTitles("=default:"))
	})
}

// TestBaseIndex checks that glaze finds windows and panes by id, so base-index and pane-base-index do not matter.
func TestBaseIndex(t *testing.T) {
	for _, tc := range []struct{ base, paneBase int }{{1, 1}, {1, 0}, {0, 1}, {5, 3}} {
		harness.Run(t, fmt.Sprintf("base_index_%d_%d", tc.base, tc.paneBase), func(c *harness.Case) {
			c.TmuxConf(fmt.Sprintf("set -g base-index %d\nset -g pane-base-index %d\n", tc.base, tc.paneBase))
			c.Fixture("baseindex/two-windows.glaze")
			c.OK(c.Up(), fmt.Sprintf("up with base-index %d pane-base-index %d", tc.base, tc.paneBase))
			w1 := c.WindowID("base-index", "w1")
			c.Equal(fmt.Sprintf("windows (bi=%d)", tc.base), "w1,w2", c.WindowNames("base-index"))
			c.Equal(fmt.Sprintf("w1 panes (pbi=%d)", tc.paneBase), "a,b", c.PaneTitles(w1))
			c.Equal(fmt.Sprintf("w2 panes (pbi=%d)", tc.paneBase), "c,d,e", c.PaneTitles(c.WindowID("base-index", "w2")))
			c.Equal("first window index", fmt.Sprint(tc.base), firstItem(c.WindowIndexes("base-index")))
			c.Equal("first pane index", fmt.Sprint(tc.paneBase), firstItem(c.PaneIndexes(w1)))
		})
	}

	harness.Run(t, "base_index_renumber", func(c *harness.Case) {
		c.TmuxConf("set -g base-index 1\nset -g renumber-windows on\n")
		c.Fixture("baseindex/two-windows.glaze")
		c.OK(c.Up(), "up with renumber-windows on")
		c.Equal("windows with renumber-windows", "w1,w2", c.WindowNames("base-index"))
	})

	// A tmux.conf that sets base-index in the background changes it while glaze provisions the session.
	harness.Run(t, "base_index_late", func(c *harness.Case) {
		c.TmuxConf(fmt.Sprintf("run-shell -b '%s -L %s set -g base-index 1'\n", c.Env().Tmux, c.Socket))
		c.Fixture("baseindex/two-windows.glaze")
		c.OK(c.Up(), "up with base-index set in the background")
		c.EventuallyEqual("both declared windows exist", "w1,w2", func() string { return c.WindowNames("base-index") })
	})
}

// firstItem returns the first item of a list that is joined with commas.
func firstItem(list string) string {
	item, _, _ := strings.Cut(list, ",")
	return item
}
