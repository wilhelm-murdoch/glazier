package cases

import (
	"fmt"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

func TestFocus(t *testing.T) {
	harness.Run(t, "focus_window_first", func(c *harness.Case) {
		c.Fixture("focus/first-window.glaze")
		c.OK(c.Up(), "up")
		c.Equal("focused first window is active", "w1", c.ActiveWindow("fw"))
	})

	harness.Run(t, "focus_window_middle", func(c *harness.Case) {
		c.Fixture("focus/middle-window.glaze")
		c.OK(c.Up(), "up")
		c.Equal("focused middle window is active", "w2", c.ActiveWindow("fw"))
	})

	harness.Run(t, "focus_window_none", func(c *harness.Case) {
		c.Fixture("focus/no-window-focus.glaze")
		c.OK(c.Up(), "up")
		c.Equal("with no focus declared the first window is active", "w1", c.ActiveWindow("fw"))
	})

	for pos := range 3 {
		harness.Run(t, fmt.Sprintf("focus_pane_%d", pos), func(c *harness.Case) {
			c.Fixture("focus/one-pane.glaze")
			c.OK(c.Up("--var", fmt.Sprintf("focused=%d", pos)), "up")
			c.Equal(fmt.Sprintf("focused pane p%d is active", pos), fmt.Sprintf("p%d", pos), c.ActivePane("=fp:w"))
		})
	}

	harness.Run(t, "focus_pane_second_window", func(c *harness.Case) {
		c.Fixture("focus/second-window.glaze")
		c.OK(c.Up(), "up")
		c.Equal("w1 focused", "w1", c.ActiveWindow("fp"))
		c.Equal("w1 pane b focused", "b", c.ActivePane("=fp:w1"))
		c.Equal("w2 pane d focused", "d", c.ActivePane("=fp:w2"))
	})

	harness.Run(t, "focus_pane_none", func(c *harness.Case) {
		c.Fixture("layouts/three-panes.glaze")
		c.OK(c.Up(), "up")
		c.Equal("with no focus declared the first pane is active", "p0", c.ActivePane("=lay:w"))
	})
}
