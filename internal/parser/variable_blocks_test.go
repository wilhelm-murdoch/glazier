package parser

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

// declaredFrom parses an in-memory profile and returns its decoded variable
// blocks alongside the diagnostics raised while decoding them.
func declaredFrom(t *testing.T, content string) ([]*Variable, hcl.Diagnostics) {
	t.Helper()
	p, diags := NewFromBytes([]byte(content), "test.glaze")
	if diags.HasErrors() {
		t.Fatalf("unexpected parse error: %s", diags)
	}
	return p.DecodeVariableBlocks()
}

func TestDecodeVariableBlocks(t *testing.T) {
	t.Run("decodes a fully specified variable", func(t *testing.T) {
		vars, diags := declaredFrom(t, `
variable "district" {
  description = "themed district"
  type        = string
  default     = "watson"
}`)
		assert.False(t, diags.HasErrors())
		assert.Len(t, vars, 1)

		v := vars[0]
		assert.Equal(t, "district", v.Name)
		assert.Equal(t, "themed district", v.Description)
		assert.Equal(t, cty.String, v.Type)
		assert.True(t, v.HasDefault)
		assert.True(t, v.Default.RawEquals(cty.StringVal("watson")))
	})

	t.Run("maps each primitive type keyword", func(t *testing.T) {
		vars, diags := declaredFrom(t, `
variable "s" { type = string }
variable "n" { type = number }
variable "b" { type = bool }`)
		assert.False(t, diags.HasErrors())

		byName := make(map[string]*Variable)
		for _, v := range vars {
			byName[v.Name] = v
		}
		assert.Equal(t, cty.String, byName["s"].Type)
		assert.Equal(t, cty.Number, byName["n"].Type)
		assert.Equal(t, cty.Bool, byName["b"].Type)
	})

	t.Run("converts a default to the declared type", func(t *testing.T) {
		// 1 is an HCL number literal; for a number variable it is kept, and the
		// number-to-string conversion lets a bare 1 stand in for "1".
		vars, diags := declaredFrom(t, `
variable "count" {
  type    = number
  default = 1
}
variable "flag" {
  type    = bool
  default = true
}`)
		assert.False(t, diags.HasErrors())

		byName := make(map[string]*Variable)
		for _, v := range vars {
			byName[v.Name] = v
		}
		assert.True(t, byName["count"].Default.RawEquals(cty.NumberIntVal(1)))
		assert.True(t, byName["flag"].Default.RawEquals(cty.True))
	})

	t.Run("records that a variable has no default", func(t *testing.T) {
		vars, diags := declaredFrom(t, `variable "x" { type = string }`)
		assert.False(t, diags.HasErrors())
		assert.False(t, vars[0].HasDefault)
		assert.Equal(t, cty.NilVal, vars[0].Default)
	})

	t.Run("collects multiple variables in declaration order", func(t *testing.T) {
		vars, diags := declaredFrom(t, `
variable "a" { type = string }
variable "b" { type = number }`)
		assert.False(t, diags.HasErrors())
		assert.Len(t, vars, 2)
		assert.Equal(t, "a", vars[0].Name)
		assert.Equal(t, "b", vars[1].Name)
	})

	t.Run("rejects an unsupported type keyword", func(t *testing.T) {
		_, diags := declaredFrom(t, `variable "x" { type = list }`)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Invalid variable type")
	})

	t.Run("rejects a quoted type", func(t *testing.T) {
		// A quoted "string" is not a bare keyword, so it is not a valid type.
		_, diags := declaredFrom(t, `variable "x" { type = "string" }`)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Invalid variable type")
	})

	t.Run("defaults a missing type to string", func(t *testing.T) {
		vars, diags := declaredFrom(t, `variable "x" { default = "y" }`)
		assert.False(t, diags.HasErrors())
		assert.Len(t, vars, 1)
		assert.Equal(t, cty.String, vars[0].Type)
		assert.True(t, vars[0].Default.RawEquals(cty.StringVal("y")))
	})

	t.Run("rejects an unknown attribute", func(t *testing.T) {
		_, diags := declaredFrom(t, `
variable "x" {
  type = string
  nope = true
}`)
		assert.True(t, diags.HasErrors())
	})

	t.Run("rejects a default that mismatches the type", func(t *testing.T) {
		_, diags := declaredFrom(t, `
variable "x" {
  type    = number
  default = "abc"
}`)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Invalid variable default")
	})

	t.Run("rejects a non-string description", func(t *testing.T) {
		_, diags := declaredFrom(t, `
variable "x" {
  description = 5
  type        = string
}`)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Invalid variable description")
	})

	t.Run("rejects a duplicate variable name", func(t *testing.T) {
		_, diags := declaredFrom(t, `
variable "x" { type = string }
variable "x" { type = number }`)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Duplicate variable")
	})

	t.Run("requires a label", func(t *testing.T) {
		p, diags := NewFromBytes([]byte(`variable { type = string }`), "test.glaze")
		assert.False(t, diags.HasErrors())
		_, blockDiags := p.DecodeVariableBlocks()
		assert.True(t, blockDiags.HasErrors())
	})

	t.Run("ignores the session block", func(t *testing.T) {
		vars, diags := declaredFrom(t, `
variable "x" { type = string }

session {
  name = "demo"
  window {
    pane {}
  }
}`)
		assert.False(t, diags.HasErrors())
		assert.Len(t, vars, 1)
	})
}

func TestResolveVariables(t *testing.T) {
	stringVar := &Variable{Name: "name", Type: cty.String}
	numberVar := &Variable{Name: "count", Type: cty.Number}
	boolVar := &Variable{Name: "on", Type: cty.Bool}

	t.Run("coerces flag values to the declared type", func(t *testing.T) {
		out, diags := ResolveVariables(
			[]*Variable{stringVar, numberVar, boolVar},
			[]string{"name=watson", "count=3", "on=true"},
			"",
			true,
		)
		assert.False(t, diags.HasErrors())
		assert.True(t, out["name"].RawEquals(cty.StringVal("watson")))
		assert.True(t, out["count"].RawEquals(cty.NumberIntVal(3)))
		assert.True(t, out["on"].RawEquals(cty.True))
	})

	t.Run("falls back to a default", func(t *testing.T) {
		withDefault := &Variable{
			Name: "name", Type: cty.String,
			Default: cty.StringVal("default"), HasDefault: true,
		}
		out, diags := ResolveVariables([]*Variable{withDefault}, nil, "", true)
		assert.False(t, diags.HasErrors())
		assert.True(t, out["name"].RawEquals(cty.StringVal("default")))
	})

	t.Run("a flag overrides a default", func(t *testing.T) {
		withDefault := &Variable{
			Name: "name", Type: cty.String,
			Default: cty.StringVal("default"), HasDefault: true,
		}
		out, diags := ResolveVariables([]*Variable{withDefault}, []string{"name=override"}, "", true)
		assert.False(t, diags.HasErrors())
		assert.True(t, out["name"].RawEquals(cty.StringVal("override")))
	})

	t.Run("reports a required variable when requireAll is set", func(t *testing.T) {
		_, diags := ResolveVariables([]*Variable{stringVar}, nil, "", true)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Required variable not set")
	})

	t.Run("omits a missing required variable when requireAll is false", func(t *testing.T) {
		out, diags := ResolveVariables([]*Variable{stringVar}, nil, "", false)
		assert.False(t, diags.HasErrors())
		_, ok := out["name"]
		assert.False(t, ok)
	})

	t.Run("reports a flag with no matching declaration", func(t *testing.T) {
		out, diags := ResolveVariables(nil, []string{"ghost=boo"}, "", false)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Undefined variable")
		assert.Empty(t, out)
	})

	t.Run("reports an undefined flag even when other variables resolve", func(t *testing.T) {
		out, diags := ResolveVariables(
			[]*Variable{stringVar},
			[]string{"name=ok", "ghost=boo"},
			"",
			true,
		)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Undefined variable")
		assert.True(t, out["name"].RawEquals(cty.StringVal("ok")))
	})

	t.Run("reports a flag value that cannot be coerced", func(t *testing.T) {
		_, diags := ResolveVariables([]*Variable{numberVar}, []string{"count=notanumber"}, "", true)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Invalid variable value")
	})
}

func TestVariableContext(t *testing.T) {
	t.Run("exposes resolved variables under the var namespace", func(t *testing.T) {
		p, diags := NewFromBytes([]byte(`variable "district" { type = string }`), "test.glaze")
		assert.False(t, diags.HasErrors())

		ctx, ctxDiags := p.VariableContext([]string{"district=watson"}, "", true)
		assert.False(t, ctxDiags.HasErrors())

		varObj := ctx.Variables["var"]
		assert.True(t, varObj.GetAttr("district").RawEquals(cty.StringVal("watson")))

		// built-ins keep their bare top-level names alongside var.
		_, hasPath := ctx.Variables["path"]
		assert.True(t, hasPath)
	})

	t.Run("var is an empty object when nothing is declared", func(t *testing.T) {
		p, diags := NewFromBytes([]byte("session {\n  name = \"demo\"\n  window {\n    pane {}\n  }\n}"), "test.glaze")
		assert.False(t, diags.HasErrors())

		ctx, ctxDiags := p.VariableContext(nil, "", true)
		assert.False(t, ctxDiags.HasErrors())
		assert.True(t, ctx.Variables["var"].RawEquals(cty.EmptyObjectVal))
	})

	t.Run("surfaces resolution diagnostics", func(t *testing.T) {
		p, diags := NewFromBytes([]byte(`variable "x" { type = string }`), "test.glaze")
		assert.False(t, diags.HasErrors())

		_, ctxDiags := p.VariableContext(nil, "", true)
		assert.True(t, ctxDiags.HasErrors())
	})
}

// TestDecodeWithVariables drives a profile with variable blocks through the
// full VariableContext + Decode pipeline, the same path `up` uses.
func TestDecodeWithVariables(t *testing.T) {
	t.Run("resolves a var reference in the session name", func(t *testing.T) {
		content := `
variable "district" {
  type    = string
  default = "watson"
}

session {
  name = "gig-${var.district}"
  window {
    pane {
      commands = ["echo"]
    }
  }
}`
		p, diags := NewFromBytes([]byte(content), "test.glaze")
		assert.False(t, diags.HasErrors())

		ctx, ctxDiags := p.VariableContext(nil, "", true)
		assert.False(t, ctxDiags.HasErrors())

		session, decodeDiags := p.Decode(spec.Session, ctx)
		assert.False(t, decodeDiags.HasErrors())
		assert.Equal(t, "gig-watson", session.Name)
	})

	t.Run("a --var value flows through to the decoded profile", func(t *testing.T) {
		content := `
variable "district" { type = string }

session {
  name = "gig-${var.district}"
  window {
    pane {
      commands = ["echo"]
    }
  }
}`
		p, _ := NewFromBytes([]byte(content), "test.glaze")
		ctx, ctxDiags := p.VariableContext([]string{"district=arasaka"}, "", true)
		assert.False(t, ctxDiags.HasErrors())

		session, decodeDiags := p.Decode(spec.Session, ctx)
		assert.False(t, decodeDiags.HasErrors())
		assert.Equal(t, "gig-arasaka", session.Name)
	})

	t.Run("a reference to an undeclared variable fails to decode", func(t *testing.T) {
		content := `
session {
  name = "gig-${var.ghost}"
  window {
    pane {
      commands = ["echo"]
    }
  }
}`
		p, _ := NewFromBytes([]byte(content), "test.glaze")
		ctx, _ := p.VariableContext(nil, "", true)

		_, decodeDiags := p.Decode(spec.Session, ctx)
		assert.True(t, decodeDiags.HasErrors())
	})
}
