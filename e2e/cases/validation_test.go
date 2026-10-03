package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// glaze rejects, before tmux starts, each value that up cannot apply.
func TestValidation(t *testing.T) {
	harness.Run(t, "validate_null_element", func(c *harness.Case) {
		c.Fixture("validation/null-element.glaze")
		r := c.Up()
		c.Fails(r, "null elements rejected")
		c.NoServer("null elements")
		c.Match("null diagnostic names the element", "must not contain null", r.Stderr)
		r = c.Glaze("format", "--validate")
		c.Fails(r, "format --validate rejects null elements")
		c.NoMatch("no panic on null elements", "panic|goroutine", r.Stderr)
	})

	harness.Run(t, "validate_hook_unknown", func(c *harness.Case) {
		c.Fixture("validation/unknown-hook.glaze")
		r := c.Up()
		c.Fails(r, "unknown hook rejected")
		c.NoServer("unknown hook")
		c.Match("unknown hook diagnostic names the hook", `"session-create"`, r.Stderr)
	})

	harness.Run(t, "validate_hook_after", func(c *harness.Case) {
		c.Fixture("validation/after-hook.glaze")
		c.OK(c.Up(), "up with an after- hook")
		c.SessionExists("after- hook session", "after-hook")
	})

	harness.Run(t, "validate_raw_layout_cells", func(c *harness.Case) {
		c.Fixture("validation/raw-layout-cells.glaze")
		r := c.Up()
		c.Fails(r, "raw layout for 2 panes on a window with 3 rejected")
		c.NoServer("raw layout cell count")
		c.Match("raw layout diagnostic gives both counts", "describes 2 panes, but the window declares 3", r.Stderr)
	})
}
