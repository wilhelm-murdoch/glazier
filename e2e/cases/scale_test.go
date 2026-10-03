package cases

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

func TestScale(t *testing.T) {
	for _, n := range []int{6, 8, 12} {
		harness.Run(t, fmt.Sprintf("scale_panes_%d", n), func(c *harness.Case) {
			var b strings.Builder
			b.WriteString("session {\n  name = \"sp\"\n  window {\n    name   = \"w\"\n    layout = \"tiled\"\n")
			for i := range n {
				fmt.Fprintf(&b, "    pane {\n      name = \"p%d\"\n    }\n", i)
			}
			b.WriteString("  }\n}\n")
			c.Write(".glaze", b.String())
			r := c.Up()
			panes := len(strings.Split(c.Tmux("list-panes", "-t", "=sp:w"), "\n"))
			c.True(fmt.Sprintf("%d panes in one window (80x24)", n), r.Code == 0 && !r.TimedOut && panes == n, "panes %d; %s", panes, r.Describe())
			c.Logf("%d panes timing: %s, session left: %t", n, r.Duration, c.HasSession("sp"))
		})
	}

	harness.Run(t, "scale_windows", func(c *harness.Case) {
		var b strings.Builder
		b.WriteString("session {\n  name = \"sw\"\n")
		for i := 1; i <= 20; i++ {
			fmt.Fprintf(&b, "  window {\n    name = \"w%d\"\n    pane {\n      commands = [\"true\", \"true\"]\n    }\n    pane {}\n  }\n", i)
		}
		b.WriteString("}\n")
		c.Write(".glaze", b.String())
		r := c.Up()
		c.OK(r, "20 windows x 2 panes")
		c.Equal("20 windows", "20", fmt.Sprint(len(strings.Split(c.Tmux("list-windows", "-t", "=sw"), "\n"))))
		c.Logf("20 windows timing: %s", r.Duration)
	})
}
