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

	// limitsLocalsProfile has the locals as %s.
	limitsLocalsProfile = `locals {
  a0 = "xxxxxxxx"
%s}
session {
  name = "limits-locals"
  window {
    pane {}
  }
}`

	// limitsLocal is one local that doubles the local before it; %[1]d is its number and %[2]d the number before.
	limitsLocal = "  a%[1]d = \"${local.a%[2]d}${local.a%[2]d}\"\n"

	// limitsNestingProfile has the opening and the closing brackets of the nested list as %s.
	limitsNestingProfile = `session {
  name = "limits-nesting"
  window {
    name = %s"x"%s
    pane {}
  }
}`
)

// glaze rejects a profile that would use too much memory or stack before it evaluates it.
func TestLimits(t *testing.T) {
	harness.Run(t, "limits_locals", func(c *harness.Case) {
		var locals strings.Builder
		for i := 1; i <= limitsDoublings; i++ {
			fmt.Fprintf(&locals, limitsLocal, i, i-1)
		}

		c.Write(".glaze", fmt.Sprintf(limitsLocalsProfile, locals.String()))
		r := c.Glaze("format", "--validate")
		c.Fails(r, "format --validate rejects a locals doubling chain")
		c.Match("the error names the local past the budget", "Locals too large", r.Stderr)
		c.Fails(c.Down(), "down rejects a locals doubling chain")
	})

	harness.Run(t, "limits_nesting", func(c *harness.Case) {
		c.Write(".glaze", fmt.Sprintf(limitsNestingProfile, strings.Repeat("[", limitsNesting), strings.Repeat("]", limitsNesting)))
		r := c.Glaze("format", "--validate")
		c.Fails(r, "format --validate rejects deep nesting")
		c.Match("the error says the nesting is too deep", "Nesting too deep", r.Stderr)
		c.Fails(c.Up(), "up rejects deep nesting")
		c.NoServer("deep nesting")
	})
}
