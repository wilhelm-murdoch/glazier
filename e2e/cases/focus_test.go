package cases

import (
	"fmt"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// focusPanes is the number of panes in focus/pane-by-index.glaze.
const focusPanes = 3

// TestFocus checks that the window and the pane with focus = true are active after up.
func TestFocus(t *testing.T) {
	for _, tc := range []struct {
		name, label, window string
		index               int
	}{
		{"focus_window_first", "focused first window is active", "w1", 0},
		{"focus_window_middle", "focused middle window is active", "w2", 1},
	} {
		harness.Run(t, tc.name, func(c *harness.Case) {
			c.Fixture("focus/window-by-index.glaze")
			c.OK(c.Up("--var", fmt.Sprintf("focused=%d", tc.index)), "up")
			c.Equal(tc.label, tc.window, c.ActiveWindow("window-by-index"))
		})
	}

	// structure/three-windows.glaze declares no focus.
	harness.Run(t, "focus_window_none", func(c *harness.Case) {
		c.Fixture("structure/three-windows.glaze")
		c.OK(c.Up(), "up")
		c.Equal("with no focus declared the first window is active", "w1", c.ActiveWindow("three-windows"))
	})

	for pos := range focusPanes {
		harness.Run(t, fmt.Sprintf("focus_pane_%d", pos), func(c *harness.Case) {
			c.Fixture("focus/pane-by-index.glaze")
			c.OK(c.Up("--var", fmt.Sprintf("focused=%d", pos)), "up")
			c.Equal(fmt.Sprintf("focused pane p%d is active", pos), fmt.Sprintf("p%d", pos), c.ActivePane("=pane-by-index:w"))
		})
	}

	harness.Run(t, "focus_pane_second_window", func(c *harness.Case) {
		c.Fixture("focus/second-window.glaze")
		c.OK(c.Up(), "up")
		c.Equal("w1 focused", "w1", c.ActiveWindow("second-window"))
		c.Equal("w1 pane b focused", "b", c.ActivePane("=second-window:w1"))
		c.Equal("w2 pane d focused", "d", c.ActivePane("=second-window:w2"))
	})

	// layouts/three-panes.glaze declares no focus.
	harness.Run(t, "focus_pane_none", func(c *harness.Case) {
		c.Fixture("layouts/three-panes.glaze")
		c.OK(c.Up(), "up")
		c.Equal("with no focus declared the first pane is active", "p0", c.ActivePane("=three-panes:w"))
	})
}
