package cases

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// firstPaneWidth reads the width of the first pane from a raw layout of side-by-side panes, which tmux puts in braces.
var firstPaneWidth = regexp.MustCompile(`\{(\d+)x`)

// TestSize checks the size and adjust blocks of a pane in a detached session of 80x24.
func TestSize(t *testing.T) {
	harness.Run(t, "size_cells_even_horizontal", func(c *harness.Case) {
		c.Fixture("size/x.glaze")
		c.OK(c.Up("--var", "x=20"), "up")
		c.Match("size x=20 honoured (layout=even-horizontal)", `^20x`, p0Size(c, "size-x"))
	})

	// tmux rounds a percentage, so the pattern allows one cell either way.
	harness.Run(t, "size_pct_even_horizontal", func(c *harness.Case) {
		c.Fixture("size/x.glaze")
		c.OK(c.Up("--var", "x=25%"), "up")
		c.Match("size x=25% honoured (layout=even-horizontal, ~20 cols)", `^(19|20|21)x`, p0Size(c, "size-x"))
	})

	harness.Run(t, "size_cells_no_layout", func(c *harness.Case) {
		c.Fixture("size/y.glaze")
		c.OK(c.Up("--var", "y=5"), "up")
		c.Match("size y=5 honoured (layout=default)", `x5$`, p0Size(c, "size-y"))
	})

	// tmux rounds a percentage, so the pattern allows one cell either way.
	harness.Run(t, "size_pct_no_layout", func(c *harness.Case) {
		c.Fixture("size/y.glaze")
		c.OK(c.Up("--var", "y=25%"), "up")
		c.Match("size y=25% honoured (layout=default, ~6 rows)", `x(5|6|7)$`, p0Size(c, "size-y"))
	})

	harness.Run(t, "size_y_even_vertical", func(c *harness.Case) {
		c.Fixture("size/y-even-vertical.glaze")
		c.OK(c.Up(), "up")
		c.Match("size y=5 honoured", `x5$`, p0Size(c, "size-y-even-vertical"))
	})

	// An even split gives p0 40 of the 80 columns, so an adjustment of 10 to the right gives it about 50.
	harness.Run(t, "size_adjust_right", func(c *harness.Case) {
		c.Fixture("size/adjust.glaze")
		c.OK(c.Up("--var", "direction=right", "--var", "amount=10"), "up")
		c.Match("adjust right 10 widens p0 beyond even split", `^(49|50|51)x`, p0Size(c, "adjust"))
	})

	harness.Run(t, "size_adjust_order", func(c *harness.Case) {
		c.Fixture("size/adjust-order.glaze")
		c.OK(c.Up(), "up")
		c.Match("size 30 then +5 -2 gives 33", `^33x`, p0Size(c, "adjust-order"))
	})

	harness.Run(t, "size_adjust_unknown_direction", func(c *harness.Case) {
		c.Fixture("size/adjust.glaze")
		r := c.Up("--var", "direction=unknown", "--var", "amount=5")
		c.Fails(r, "adjust direction 'unknown' rejected")
		c.NoServer("unknown direction")
		c.Match("unknown direction diagnostic", `not supported`, r.Stderr)
	})

	harness.Run(t, "size_adjust_bad_direction", func(c *harness.Case) {
		c.Fixture("size/adjust.glaze")
		c.Fails(c.Up("--var", "direction=sideways", "--var", "amount=5"), "adjust direction 'sideways' rejected")
		c.NoServer("bad direction")
	})

	harness.Run(t, "size_adjust_five_blocks", func(c *harness.Case) {
		c.Fixture("size/five-adjust.glaze")
		c.Fails(c.Up(), "five adjust blocks rejected")
		c.NoServer("five adjust")
	})

	// glaze rejects each value before it starts tmux, so the values share one case.
	harness.Run(t, "size_invalid", func(c *harness.Case) {
		c.Fixture("size/x.glaze")
		for _, v := range []string{"0", "-5", "abc", "101%", "0%", "", " 10", "10.5", "1e3", "%"} {
			c.Fails(c.Up("--var", "x="+v), fmt.Sprintf("size x=[%s] rejected", v))
			c.NoServer(fmt.Sprintf("size x=[%s]", v))
		}
	})

	// glaze rejects each value before it starts tmux, so the values share one case.
	harness.Run(t, "size_adjust_amount_invalid", func(c *harness.Case) {
		c.Fixture("size/adjust.glaze")
		for _, v := range []string{"0", "-3", "abc", "10%"} {
			c.Fails(c.Up("--var", "direction=left", "--var", "amount="+v), fmt.Sprintf("adjust amount=[%s] rejected", v))
			c.NoServer(fmt.Sprintf("adjust amount=[%s]", v))
		}
	})

	// A raw layout string fixes the size of every pane, so glaze ignores size and warns.
	harness.Run(t, "size_raw_layout", func(c *harness.Case) {
		raw := rawLayout(c, 2)
		m := firstPaneWidth.FindStringSubmatch(raw)
		if m == nil {
			c.T().Fatalf("no pane width in the raw layout %q", raw)
		}

		c.Fixture("size/x.glaze")
		r := c.Up("--var", "layout="+raw, "--var", "x=20")
		c.OK(r, "up with a raw layout and a size")
		c.Match("the raw layout keeps its width", "^"+m[1]+"x", p0Size(c, "size-x"))
		c.Match("up warns that it ignores size", `ignores size and adjust`, r.Stderr)
	})

	harness.Run(t, "size_empty_block", func(c *harness.Case) {
		c.Fixture("size/empty-size.glaze")
		c.Fails(c.Up(), "empty size block rejected")
		c.NoServer("empty size")
	})

	// tmux keeps one column for p1 and one for the border, so p0 gets the rest of the 80 columns.
	harness.Run(t, "size_huge", func(c *harness.Case) {
		c.Fixture("size/x.glaze")
		c.OK(c.Up("--var", "x=5000"), "size larger than window")
		c.Equal("a size larger than the window takes all the space that tmux gives", "78x24", p0Size(c, "size-x"))
	})
}

// p0Size returns WIDTHxHEIGHT of pane p0 in window w of a size fixture.
func p0Size(c *harness.Case, session string) string {
	c.T().Helper()
	return c.PaneSize("="+session+":w", "p0")
}
