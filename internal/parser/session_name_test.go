package parser

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

// profileWithName returns a profile whose session `name` line is nameLine, after the given top-level blocks.
func profileWithName(blocks, nameLine string) string {
	return blocks + "\nsession {\n  " + nameLine + "\n  window {\n    pane {}\n  }\n}\n"
}

// resolvedContext returns the context that `up` and `format --validate` give the registry name check, with every variable and local resolved.
func resolvedContext(t *testing.T, p *Parser) *hcl.EvalContext {
	t.Helper()

	ctx, diags := p.VariableContext(nil, "", true)
	require.False(t, diags.HasErrors(), diags.Error())

	return ctx
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
			p := parse(t, profileWithName(tc.blocks, tc.nameLine))

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
		p := parse(t, profileWithName("", `name = ["a"]`))

		ctx, _ := p.VariableContext(nil, "", true)

		_, diags := p.DecodeSessionName(spec.SessionName, ctx)
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
			p := parse(t, profileWithName(tc.blocks, tc.nameLine))

			ctx, _ := p.VariableContext(nil, "", true)

			_, diags := p.DecodeSessionName(spec.SessionName, ctx)
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
		p := parse(t, profile)

		ctx, _ := p.VariableContext(nil, "", true)

		name, diags := p.DecodeSessionName(spec.SessionName, ctx)
		assert.False(t, diags.HasErrors(), diags.Error())
		assert.Equal(t, "demo", name)
	})

	t.Run("stops at a loop between locals", func(t *testing.T) {
		p := parse(t, profileWithName("locals {\n  a = local.b\n  b = local.a\n}", "name = local.a"))

		block, diags := p.sessionBlock()
		assert.False(t, diags.HasErrors())
		assert.Empty(t, p.checkSessionName(block))
	})
}

func TestCheckRegistryName(t *testing.T) {
	t.Run("passes a path profile, which has no profile name", func(t *testing.T) {
		p := parse(t, profileWithName("", ""))
		assert.Empty(t, p.CheckRegistryName("", nil))
	})

	for _, tc := range []struct {
		name     string
		blocks   string
		nameLine string
	}{
		{"a string", "", `name = "demo"`},
		{"a number", "", "name = 42"},
		{"a local", `locals { n = "demo" }`, "name = local.n"},
		{"a template from path", "", `name = "reg-${path.base}"`},
		{"a variable with a default", `variable "d" { default = "demo" }`, "name = var.d"},
	} {
		t.Run("passes "+tc.name, func(t *testing.T) {
			p := parse(t, profileWithName(tc.blocks, tc.nameLine))
			assert.Empty(t, p.CheckRegistryName("work", resolvedContext(t, p)))
		})
	}

	t.Run("passes an expression that the context cannot evaluate, which the decode reports", func(t *testing.T) {
		// Without a context the local has no value, so the check cannot see the null and leaves the error to the decode.
		p := parse(t, profileWithName(`locals { n = null }`, "name = local.n"))
		assert.Empty(t, p.CheckRegistryName("work", nil))
	})

	t.Run("passes a variable without a value, which the decode reports", func(t *testing.T) {
		// `down` resolves only the variables it can, so its context has no var.d and the check cannot evaluate the name.
		p := parse(t, profileWithName(`variable "d" {}`, "name = var.d"))

		ctx, diags := p.VariableContext(nil, "", false)
		require.False(t, diags.HasErrors(), diags.Error())
		assert.Empty(t, p.CheckRegistryName("work", ctx))
	})

	t.Run("rejects a missing name and points at the session block", func(t *testing.T) {
		p := parse(t, profileWithName("", ""))

		diags := p.CheckRegistryName("work", resolvedContext(t, p))
		if assert.True(t, diags.HasErrors()) {
			assert.Equal(t, "Missing session name", diags[0].Summary)
			assert.Contains(t, diags[0].Detail, "The profile `work` is in the profile registry, so its session must have a name. Set name in the session block.")
			assert.Contains(t, diags[0].Detail, "Without it the session is named default, and `glaze down work` could kill an unrelated session.")
			assert.Equal(t, 2, diags[0].Subject.Start.Line)
		}
	})

	for _, tc := range []struct {
		name     string
		blocks   string
		nameLine string
	}{
		{"a null name", "", "name = null"},
		{"a null local", `locals { n = null }`, "name = local.n"},
		{"a variable with a null default", `variable "d" { default = null }`, "name = var.d"},
	} {
		t.Run("rejects "+tc.name+" and points at the name", func(t *testing.T) {
			p := parse(t, profileWithName(tc.blocks, tc.nameLine))

			// A null local or variable reaches the name only through the context, which the action resolves before the check.
			diags := p.CheckRegistryName("work", resolvedContext(t, p))
			if assert.True(t, diags.HasErrors()) {
				assert.Equal(t, "Missing session name", diags[0].Summary)
				assert.Equal(t, 3, diags[0].Subject.Start.Line, "the name line of the profile")
			}
		})
	}

	t.Run("passes a profile without a session block, which the decode reports", func(t *testing.T) {
		p := parse(t, `locals { n = "demo" }`)
		assert.Empty(t, p.CheckRegistryName("work", nil))
	})
}

func TestSessionNameUsesPath(t *testing.T) {
	for _, tc := range []struct {
		name     string
		blocks   string
		nameLine string
		want     bool
	}{
		{"path.base in a template", "", `name = "reg-${path.base}"`, true},
		{"path.pwd", "", "name = path.pwd", true},
		{"path through a local", "locals {\n  dir = path.base\n  n   = \"reg-${local.dir}\"\n}", "name = local.n", true},
		{"a string", "", `name = "demo"`, false},
		{"a variable", `variable "d" {}`, `name = "gig-${var.d}"`, false},
		{"a local without path", `locals { n = "demo" }`, "name = local.n", false},
		{"a missing name", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := parse(t, profileWithName(tc.blocks, tc.nameLine))
			assert.Equal(t, tc.want, p.SessionNameUsesPath())
		})
	}

	t.Run("ignores path in a window name", func(t *testing.T) {
		profile := "session {\n  name = \"demo\"\n  window {\n    name = path.base\n    pane {}\n  }\n}\n"
		p := parse(t, profile)
		assert.False(t, p.SessionNameUsesPath())
	})

	t.Run("is false without a session block", func(t *testing.T) {
		p := parse(t, `locals { n = path.base }`)
		assert.False(t, p.SessionNameUsesPath())
	})
}
