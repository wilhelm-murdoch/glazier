package cases

import (
	"fmt"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

func TestStructure(t *testing.T) {
	harness.Run(t, "struct_basic", func(c *harness.Case) {
		c.Fixture("structure/three-windows.glaze")
		c.OK(c.Up(), "up")
		c.Equal("window order", "w1,w2,w3", c.WindowNames("st"))
		c.Equal("window indexes start at base-index 0", "0,1,2", commas(c.Tmux("list-windows", "-t", "=st", "-F", "#{window_index}")))
		c.Equal("w1 pane order", "a,b,c", c.PaneTitles(c.WindowID("st", "w1")))
		c.Equal("w1 pane indexes", "0,1,2", commas(c.Tmux("list-panes", "-t", c.WindowID("st", "w1"), "-F", "#{pane_index}")))
		c.Equal("w2 panes", "d", c.PaneTitles(c.WindowID("st", "w2")))
		c.Equal("w3 pane order", "e,f", c.PaneTitles(c.WindowID("st", "w3")))
		c.Equal("one session only", "1", fmt.Sprint(len(c.Sessions())))
		c.Snapshot("st")
	})

	harness.Run(t, "struct_defaults", func(c *harness.Case) {
		c.Fixture("structure/no-names.glaze")
		c.OK(c.Up(), "up with no names")
		c.SessionExists("session name defaults to 'default'", "default")
		c.Equal("window name defaults to 'default'", "default", c.WindowNames("default"))
		c.Equal("pane names default to 'default'", "default,default", c.PaneTitles("=default:"))
	})
}

// glaze never reads base-index or pane-base-index; it finds windows and panes by id.
func TestBaseIndex(t *testing.T) {
	for _, cfg := range [][2]int{{1, 1}, {1, 0}, {0, 1}, {5, 3}} {
		bi, pbi := cfg[0], cfg[1]
		harness.Run(t, fmt.Sprintf("base_index_%d_%d", bi, pbi), func(c *harness.Case) {
			c.TmuxConf(fmt.Sprintf("set -g base-index %d\nset -g pane-base-index %d\n", bi, pbi))
			c.Fixture("baseindex/two-windows.glaze")
			c.OK(c.Up(), fmt.Sprintf("up with base-index %d pane-base-index %d", bi, pbi))
			c.Equal(fmt.Sprintf("windows (bi=%d)", bi), "w1,w2", c.WindowNames("bi"))
			c.Equal(fmt.Sprintf("w1 panes (pbi=%d)", pbi), "a,b", c.PaneTitles(c.WindowID("bi", "w1")))
			c.Equal(fmt.Sprintf("w2 panes (pbi=%d)", pbi), "c,d,e", c.PaneTitles(c.WindowID("bi", "w2")))
			c.Equal("first window index", fmt.Sprint(bi), firstLine(c.Tmux("list-windows", "-t", "=bi", "-F", "#{window_index}")))
			c.Equal("first pane index", fmt.Sprint(pbi), firstLine(c.Tmux("list-panes", "-t", c.WindowID("bi", "w1"), "-F", "#{pane_index}")))
		})
	}

	harness.Run(t, "base_index_renumber", func(c *harness.Case) {
		c.TmuxConf("set -g base-index 1\nset -g renumber-windows on\n")
		c.Fixture("baseindex/renumber.glaze")
		c.OK(c.Up(), "up with renumber-windows on")
		c.Equal("windows with renumber-windows", "w1,w2", c.WindowNames("rn"))
	})

	harness.Run(t, "base_index_late", func(c *harness.Case) {
		// A tmux.conf that sets base-index in the background changes it while glaze provisions the session.
		c.TmuxConf(fmt.Sprintf("run-shell -b '%s -L %s set -g base-index 1'\n", c.Env().Tmux, c.Socket))
		c.Fixture("baseindex/late.glaze")
		c.OK(c.Up(), "up with base-index set in the background")
		c.EventuallyEqual("both declared windows exist", "one,two", func() string { return c.WindowNames("late") })
	})
}
