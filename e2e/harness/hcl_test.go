package harness

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// TestQuoteRoundTrip parses each literal with the HCL parser and checks that
// it reads back as exactly the original string.
func TestQuoteRoundTrip(t *testing.T) {
	for _, s := range []string{
		"", "plain", `quote " and \ backslash`, "tab\tnewline\ncr\r", "${var.x}", "%{ if x }", "$$ and %%",
		"bell\a esc\x1b del\x7f", "ünïcödé ✓", "trailing $", "trailing %", "$", "${", "semi;colon",
	} {
		expr, diags := hclsyntax.ParseExpression([]byte(Quote(s)), "q.hcl", hcl.InitialPos)
		if diags.HasErrors() {
			t.Errorf("%q: %s", s, diags.Error())
			continue
		}
		v, diags := expr.Value(nil)
		if diags.HasErrors() {
			t.Errorf("%q: %s", s, diags.Error())
			continue
		}
		if got := v.AsString(); got != s {
			t.Errorf("Quote(%q) reads back as %q", s, got)
		}
	}
}

func TestList(t *testing.T) {
	if got, want := List("a", `b"`), `["a", "b\""]`; got != want {
		t.Errorf("List: %s, want %s", got, want)
	}
}
