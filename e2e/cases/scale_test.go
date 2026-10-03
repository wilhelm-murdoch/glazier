package cases

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

const (
	// scaleWindows is the number of windows in scale_windows.
	scaleWindows = 20

	// scalePanesProfile has the panes of one tiled window as %s.
	scalePanesProfile = `session {
  name = "scale-panes"
  window {
    name   = "w"
    layout = "tiled"
%s  }
}`

	// scalePane is one pane of scalePanesProfile; %d is its number.
	scalePane = `    pane {
      name = "p%d"
    }`

	// scaleWindowsProfile has the windows as %s.
	scaleWindowsProfile = `session {
  name = "scale-windows"
%s}
`

	// scaleWindow is one window of scaleWindowsProfile with two panes; %d is its number.
	scaleWindow = `  window {
    name = "w%d"
    pane {
      commands = ["true", "true"]
    }
    pane {}
  }`
)

// up builds many panes in one window and many windows in one session.
func TestScale(t *testing.T) {
	for _, panes := range []int{6, 8, 12} {
		harness.Run(t, fmt.Sprintf("scale_panes_%d", panes), func(c *harness.Case) {
			var blocks strings.Builder
			for i := range panes {
				fmt.Fprintf(&blocks, scalePane+"\n", i)
			}

			c.Write(".glaze", fmt.Sprintf(scalePanesProfile, blocks.String()))
			r := c.Up()
			got := c.PaneCount("=scale-panes:w")
			c.True(fmt.Sprintf("%d panes in one window (80x24)", panes), r.Succeeded() && got == panes, "panes %d; %s", got, r.Describe())
			c.Logf("%d panes timing: %s, session left: %t", panes, r.Duration, c.HasSession("scale-panes"))
		})
	}

	harness.Run(t, "scale_windows", func(c *harness.Case) {
		var blocks strings.Builder
		for i := 1; i <= scaleWindows; i++ {
			fmt.Fprintf(&blocks, scaleWindow+"\n", i)
		}

		c.Write(".glaze", fmt.Sprintf(scaleWindowsProfile, blocks.String()))
		r := c.Up()
		c.OK(r, fmt.Sprintf("%d windows x 2 panes", scaleWindows))
		c.Equal(fmt.Sprintf("%d windows", scaleWindows), strconv.Itoa(scaleWindows), strconv.Itoa(c.WindowCount("scale-windows")))
		c.Logf("%d windows timing: %s", scaleWindows, r.Duration)
	})
}
