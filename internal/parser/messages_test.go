package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

// summaries returns the summary of each diagnostic, so a test can check which errors a profile gets.
func summaries(t *testing.T, profile string, requireAll bool) []string {
	t.Helper()

	p, diags := New(writeGlaze(t, profile))
	require.False(t, diags.HasErrors(), diags.Error())

	_, diags = p.VariableContext(nil, "", requireAll)

	var out []string
	for _, diag := range diags {
		out = append(out, diag.Summary+": "+diag.Detail)
	}

	return out
}

func TestLocalsCycleIsOneError(t *testing.T) {
	t.Run("a cycle of three locals", func(t *testing.T) {
		got := summaries(t, profileWithName("locals {\n  a = local.b\n  b = local.c\n  c = local.a\n}", `name = "demo"`), true)
		require.Len(t, got, 1, strings.Join(got, "\n"))
		assert.Contains(t, got[0], "Circular reference between locals")
		assert.Contains(t, got[0], "local.a, local.b, local.c")
	})

	t.Run("a local that only depends on a cycle gets no error of its own", func(t *testing.T) {
		got := summaries(t, profileWithName("locals {\n  a = local.b\n  b = local.a\n  user = upper(local.a)\n}", `name = "demo"`), true)
		require.Len(t, got, 1, strings.Join(got, "\n"))
		assert.Contains(t, got[0], "local.a, local.b")
		assert.NotContains(t, got[0], "local.user")
	})

	t.Run("two cycles are two errors", func(t *testing.T) {
		got := summaries(t, profileWithName("locals {\n  a = local.b\n  b = local.a\n  x = local.x\n}", `name = "demo"`), true)
		require.Len(t, got, 2, strings.Join(got, "\n"))
		assert.Contains(t, got[0], "local.a, local.b")
		assert.Contains(t, got[1], "local.x")
	})

	t.Run("an error that is not a cycle is still reported", func(t *testing.T) {
		got := summaries(t, profileWithName("locals {\n  a = local.missing\n}", `name = "demo"`), true)
		require.Len(t, got, 1)
		assert.Contains(t, got[0], "Unsupported attribute")
	})
}

func TestDecodeSessionNameNamesTheMissingVariable(t *testing.T) {
	const variables = "variable \"region\" {\n  type = string\n}\nvariable \"unused\" {\n  type = string\n}\n"

	for _, tc := range []struct {
		name     string
		blocks   string
		nameLine string
	}{
		{"used directly", variables, `name = "svc-${var.region}"`},
		{"used through a local", variables + "locals {\n  base = \"svc-${var.region}\"\n}", "name = local.base"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, diags := New(writeGlaze(t, profileWithName(tc.blocks, tc.nameLine)))
			require.False(t, diags.HasErrors())

			// `down` resolves only what it can, so var.region has no value here.
			ctx, _ := p.VariableContext(nil, "", false)

			_, diags = p.DecodeSessionName(spec.SessionName, ctx)
			require.True(t, diags.HasErrors())
			require.Len(t, diags, 1, diags.Error())
			assert.Equal(t, "Required variable not set", diags[0].Summary)
			assert.Contains(t, diags[0].Detail, `"region"`)
		})
	}
}
