package cases

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

const (
	// limitsDoublings is the length of a chain of locals that each double the one before: 22 steps of 8 bytes is 32 MiB.
	// This is small enough for a build of glaze without the limit.
	limitsDoublings = 22
	// limitsNesting is the depth of a nested list, past the limit of 256 levels.
	limitsNesting = 1000
)

func TestLimits(t *testing.T) {
	harness.Run(t, "limits_locals", func(c *harness.Case) {
		var b strings.Builder
		b.WriteString("locals {\n  a0 = \"xxxxxxxx\"\n")
		for i := 1; i <= limitsDoublings; i++ {
			fmt.Fprintf(&b, "  a%d = \"${local.a%d}${local.a%d}\"\n", i, i-1, i-1)
		}
		b.WriteString("}\nsession {\n  name = \"lb\"\n  window {\n    pane {}\n  }\n}\n")
		c.Write(".glaze", b.String())
		r := c.Glaze("format", "--validate")
		c.Fails(r, "format --validate rejects a locals doubling chain")
		c.Match("the error names the local past the budget", "Locals too large", r.Stderr)
		c.Fails(c.Down(), "down rejects a locals doubling chain")
	})

	harness.Run(t, "limits_nesting", func(c *harness.Case) {
		c.Write(".glaze", fmt.Sprintf("session {\n  name = \"ln\"\n  window {\n    name = %s\"x\"%s\n    pane {}\n  }\n}\n",
			strings.Repeat("[", limitsNesting), strings.Repeat("]", limitsNesting)))
		r := c.Glaze("format", "--validate")
		c.Fails(r, "format --validate rejects deep nesting")
		c.Match("the error says the nesting is too deep", "Nesting too deep", r.Stderr)
		c.Fails(c.Up(), "up rejects deep nesting")
		c.NoServer("deep nesting")
	})
}
