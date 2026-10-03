package cases

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// sizeTarget is the window of every size fixture.
const sizeTarget = "=sz:w"

// sizeFirstWidth finds the width of the first pane in a raw layout string.
var sizeFirstWidth = regexp.MustCompile(`\{(\d+)x`)

func TestSize(t *testing.T) {
	harness.Run(t, "size_cells_even-horizontal", func(c *harness.Case) {
		c.Fixture("size/x.glaze")
		c.OK(c.Up("--var", "x=20"), "up")
		c.Match("size x=20 honoured (layout=even-horizontal)", `^20x`, c.PaneSize(sizeTarget, "p0"))
	})

	harness.Run(t, "size_pct_even-horizontal", func(c *harness.Case) {
		c.Fixture("size/x.glaze")
		c.OK(c.Up("--var", "x=25%"), "up")
		c.Match("size x=25% honoured (layout=even-horizontal, ~20 cols)", `^(19|20|21)x`, c.PaneSize(sizeTarget, "p0"))
	})

	harness.Run(t, "size_cells_nolayout", func(c *harness.Case) {
		c.Fixture("size/y.glaze")
		c.OK(c.Up("--var", "y=5"), "up")
		c.Match("size y=5 honoured (layout=default)", `x5$`, c.PaneSize(sizeTarget, "p0"))
	})

	harness.Run(t, "size_pct_nolayout", func(c *harness.Case) {
		c.Fixture("size/y.glaze")
		c.OK(c.Up("--var", "y=25%"), "up")
		c.Match("size y=25% honoured (layout=default, ~6 rows)", `x(5|6|7)$`, c.PaneSize(sizeTarget, "p0"))
	})

	harness.Run(t, "size_y", func(c *harness.Case) {
		c.Fixture("size/y-even-vertical.glaze")
		c.OK(c.Up(), "up")
		c.Match("size y=5 honoured", `x5$`, c.PaneSize(sizeTarget, "p0"))
	})

	harness.Run(t, "adjust_right", func(c *harness.Case) {
		c.Fixture("size/adjust.glaze")
		c.OK(c.Up("--var", "direction=right", "--var", "amount=10"), "up")
		size := c.PaneSize(sizeTarget, "p0")
		c.Logf("adjust right 10 width (even split is 40): %s", size)
		c.Match("adjust right 10 widens p0 beyond even split", `^(49|50|51)x`, size)
	})

	harness.Run(t, "adjust_order", func(c *harness.Case) {
		c.Fixture("size/adjust-order.glaze")
		c.OK(c.Up(), "up")
		c.Match("size 30 then +5 -2 gives 33", `^33x`, c.PaneSize(sizeTarget, "p0"))
	})

	harness.Run(t, "adjust_unknown_direction", func(c *harness.Case) {
		c.Fixture("size/adjust.glaze")
		r := c.Up("--var", "direction=unknown", "--var", "amount=5")
		c.Fails(r, "adjust direction 'unknown' rejected")
		c.NoServer("unknown direction")
		c.Match("unknown direction diagnostic", `not supported`, r.Stderr)
	})

	harness.Run(t, "adjust_bad_direction", func(c *harness.Case) {
		c.Fixture("size/adjust.glaze")
		c.Fails(c.Up("--var", "direction=sideways", "--var", "amount=5"), "adjust direction 'sideways' rejected")
		c.NoServer("bad direction")
	})

	harness.Run(t, "adjust_five_blocks", func(c *harness.Case) {
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

	harness.Run(t, "adjust_amount_invalid", func(c *harness.Case) {
		c.Fixture("size/adjust.glaze")
		for _, v := range []string{"0", "-3", "abc", "10%"} {
			c.Fails(c.Up("--var", "direction=left", "--var", "amount="+v), fmt.Sprintf("adjust amount=[%s] rejected", v))
		}
	})

	// A raw layout string fixes the size of every pane, so glaze ignores size and warns.
	harness.Run(t, "size_raw_layout", func(c *harness.Case) {
		raw := rawLayout(c, 2)
		m := sizeFirstWidth.FindStringSubmatch(raw)
		if m == nil {
			c.T().Fatalf("no pane width in the raw layout %q", raw)
		}
		c.Fixture("size/x.glaze")
		r := c.Up("--var", "layout="+raw, "--var", "x=20")
		c.OK(r, "up with a raw layout and a size")
		c.Match("the raw layout keeps its width", "^"+m[1]+"x", c.PaneSize(sizeTarget, "p0"))
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
		c.Equal("a size larger than the window takes all the space that tmux gives", "78x24", c.PaneSize(sizeTarget, "p0"))
	})
}
