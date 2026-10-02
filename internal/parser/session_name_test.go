package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

// profileWithName returns a profile whose session `name` line is nameLine, after the given top-level blocks.
func profileWithName(blocks, nameLine string) string {
	return blocks + "\nsession {\n  " + nameLine + "\n  window {\n    pane {}\n  }\n}\n"
}

func TestDecodeSessionNameMatchesDecode(t *testing.T) {
	for _, tc := range []struct {
		name     string
		blocks   string
		nameLine string
		want     string
	}{
		{"a string", "", `name = "demo"`, "demo"},
		{"a missing name", "", "", "default"},
		{"a null name", "", "name = null", "default"},
		{"a number", "", "name = 42", "42"},
		{"a bool", "", "name = true", "true"},
		{"a local", `locals { n = "from-local" }`, "name = local.n", "from-local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, diags := New(writeGlaze(t, profileWithName(tc.blocks, tc.nameLine)))
			assert.False(t, diags.HasErrors())

			ctx, diags := p.VariableContext(nil, "", true)
			assert.False(t, diags.HasErrors())

			down, diags := p.DecodeSessionName(spec.SessionName, ctx)
			assert.False(t, diags.HasErrors(), diags.Error())
			assert.Equal(t, tc.want, down)

			// `up` and `down` must agree, or `down` looks for a different session than the one that `up` created.
			session, diags := p.Decode(spec.Session(""), ctx)
			assert.False(t, diags.HasErrors(), diags.Error())
			assert.Equal(t, session.Name, down)
		})
	}

	t.Run("rejects a list, the same as up", func(t *testing.T) {
		p, diags := New(writeGlaze(t, profileWithName("", `name = ["a"]`)))
		assert.False(t, diags.HasErrors())

		ctx, _ := p.VariableContext(nil, "", true)

		_, diags = p.DecodeSessionName(spec.SessionName, ctx)
		assert.True(t, diags.HasErrors())

		_, diags = p.Decode(spec.Session(""), ctx)
		assert.True(t, diags.HasErrors())
	})
}

func TestSessionNameRejectsRandom(t *testing.T) {
	for _, tc := range []struct {
		name     string
		blocks   string
		nameLine string
	}{
		{"a direct call", "", `name = random(["a", "b"])`},
		{"a call inside a template", "", `name = "gig-${random(["a", "b"])}"`},
		{"a local", `locals { pick = random(["a", "b"]) }`, "name = local.pick"},
		{"a chain of locals", "locals {\n  pick = random([\"a\", \"b\"])\n  name = upper(local.pick)\n}", `name = "gig-${local.name}"`},
		{"a local with a for expression", "locals {\n  editors = [\"nvim\", \"hx\"]\n  pick = random([for e in local.editors : e])\n}", "name = local.pick"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, diags := New(writeGlaze(t, profileWithName(tc.blocks, tc.nameLine)))
			assert.False(t, diags.HasErrors())

			ctx, _ := p.VariableContext(nil, "", true)

			_, diags = p.DecodeSessionName(spec.SessionName, ctx)
			assert.True(t, diags.HasErrors())
			assert.Contains(t, diags.Error(), "must not use random()")

			_, diags = p.Decode(spec.Session(""), ctx)
			assert.True(t, diags.HasErrors())
			assert.Contains(t, diags.Error(), "must not use random()")
		})
	}

	t.Run("allows random() in a window name and a local that the name does not use", func(t *testing.T) {
		profile := "locals {\n  pick = random([\"a\", \"b\"])\n  fixed = \"demo\"\n}\n" +
			"session {\n  name = local.fixed\n  window {\n    name = local.pick\n    pane {}\n  }\n}\n"
		p, diags := New(writeGlaze(t, profile))
		assert.False(t, diags.HasErrors())

		ctx, _ := p.VariableContext(nil, "", true)

		name, diags := p.DecodeSessionName(spec.SessionName, ctx)
		assert.False(t, diags.HasErrors(), diags.Error())
		assert.Equal(t, "demo", name)
	})

	t.Run("stops at a loop between locals", func(t *testing.T) {
		p, diags := New(writeGlaze(t, profileWithName("locals {\n  a = local.b\n  b = local.a\n}", "name = local.a")))
		assert.False(t, diags.HasErrors())

		block, diags := p.sessionBlock()
		assert.False(t, diags.HasErrors())
		assert.Empty(t, p.checkSessionName(block))
	})
}
