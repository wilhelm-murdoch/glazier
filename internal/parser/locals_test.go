package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

// localsContext builds the full VariableContext for an in-memory profile, the
// same entry point the CLI actions use, so locals are exercised alongside the
// var/env/path namespaces they may reference.
func localsContext(t *testing.T, content string, flags []string, requireAll bool) (map[string]cty.Value, bool, string) {
	t.Helper()

	p, diags := NewFromBytes([]byte(content), "test.glaze")
	require.False(t, diags.HasErrors(), "unexpected parse error: %s", diags)

	ctx, ctxDiags := p.VariableContext(flags, "", requireAll)

	locals := map[string]cty.Value{}
	if obj, ok := ctx.Variables["local"]; ok && obj.Type().IsObjectType() {
		locals = obj.AsValueMap()
	}

	return locals, ctxDiags.HasErrors(), ctxDiags.Error()
}

func TestResolveLocals(t *testing.T) {
	t.Run("resolves a literal local", func(t *testing.T) {
		locals, hasErrors, _ := localsContext(t, `
locals {
  slug = "watson"
}`, nil, true)
		assert.False(t, hasErrors)
		assert.True(t, locals["slug"].RawEquals(cty.StringVal("watson")))
	})

	t.Run("locals may reference variables and functions", func(t *testing.T) {
		locals, hasErrors, _ := localsContext(t, `
variable "district" { default = "Night City" }

locals {
  slug = lower(replace(var.district, " ", "-"))
}`, nil, true)
		assert.False(t, hasErrors)
		assert.True(t, locals["slug"].RawEquals(cty.StringVal("night-city")))
	})

	t.Run("locals may reference each other in any declaration order", func(t *testing.T) {
		locals, hasErrors, _ := localsContext(t, `
locals {
  session = "gig-${local.slug}"
  slug    = lower(local.raw)
  raw     = "WATSON"
}`, nil, true)
		assert.False(t, hasErrors)
		assert.True(t, locals["session"].RawEquals(cty.StringVal("gig-watson")))
	})

	t.Run("locals across multiple blocks share one namespace", func(t *testing.T) {
		locals, hasErrors, _ := localsContext(t, `
locals {
  first = "a"
}

locals {
  second = "${local.first}b"
}`, nil, true)
		assert.False(t, hasErrors)
		assert.True(t, locals["second"].RawEquals(cty.StringVal("ab")))
	})

	t.Run("rejects a duplicate local name", func(t *testing.T) {
		_, hasErrors, detail := localsContext(t, `
locals {
  slug = "a"
}

locals {
  slug = "b"
}`, nil, true)
		assert.True(t, hasErrors)
		assert.Contains(t, detail, "Duplicate local value")
	})

	t.Run("an unresolvable local reports its own evaluation error", func(t *testing.T) {
		_, hasErrors, detail := localsContext(t, `
locals {
  slug = var.ghost
}`, nil, true)
		assert.True(t, hasErrors)
		assert.Contains(t, detail, "ghost")
	})

	t.Run("a reference cycle fails rather than looping", func(t *testing.T) {
		_, hasErrors, _ := localsContext(t, `
locals {
  a = local.b
  b = local.a
}`, nil, true)
		assert.True(t, hasErrors)
	})

	t.Run("a lenient pass drops unresolved locals silently", func(t *testing.T) {
		// requireAll=false mirrors `down`: a broken local nobody references
		// must not block the teardown.
		locals, hasErrors, _ := localsContext(t, `
locals {
  ok     = "fine"
  broken = var.ghost
}`, nil, false)
		assert.False(t, hasErrors)
		assert.True(t, locals["ok"].RawEquals(cty.StringVal("fine")))
		_, resolved := locals["broken"]
		assert.False(t, resolved)
	})
}

// TestDecodeWithLocals drives locals through the full VariableContext + Decode
// pipeline, the same path `up` uses.
func TestDecodeWithLocals(t *testing.T) {
	content := `
variable "district" { default = "Watson" }

locals {
  slug = lower(var.district)
}

session {
  name = "gig-${local.slug}"
  window {
    pane {
      commands = ["echo"]
    }
  }
}`
	p, diags := NewFromBytes([]byte(content), "test.glaze")
	require.False(t, diags.HasErrors())

	ctx, ctxDiags := p.VariableContext(nil, "", true)
	require.False(t, ctxDiags.HasErrors(), "unexpected diagnostics: %s", ctxDiags)

	session, decodeDiags := p.Decode(spec.Session, ctx)
	require.False(t, decodeDiags.HasErrors())
	assert.Equal(t, "gig-watson", session.Name)
}
