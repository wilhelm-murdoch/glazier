package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func TestCollectEnvVariables(t *testing.T) {
	envs := []string{
		"GLAZE_ENV_FOO=bar",
		"GLAZE_ENV_EMPTY=",
		"PATH=/usr/bin",         // wrong prefix, ignored
		"GLAZE_ENV_NOEQUALSIGN", // no "=", ignored
		"GLAZE_ENV_MULTI=a=b=c", // only split on first "="
	}

	out := collectEnvVariables(envs, EnvVariablePrefix)

	assert.Equal(t, cty.StringVal("bar"), out["FOO"])
	assert.Equal(t, cty.StringVal(""), out["EMPTY"])
	assert.Equal(t, cty.StringVal("a=b=c"), out["MULTI"])
	_, hasPath := out["PATH"]
	assert.False(t, hasPath)
	_, hasNoEq := out["NOEQUALSIGN"]
	assert.False(t, hasNoEq)
}

func TestCollectFlagVariables(t *testing.T) {
	out := collectFlagVariables([]string{
		"region=us-east-1",
		"  spaced  =value",
		"token=a=b",
	})

	assert.Equal(t, cty.StringVal("us-east-1"), out["region"])
	assert.Equal(t, cty.StringVal("value"), out["spaced"])
	assert.Equal(t, cty.StringVal("a=b"), out["token"])
}

func TestAddDefaultVariables(t *testing.T) {
	out, err := addDefaultVariables()
	assert.NoError(t, err)

	path, ok := out["path"]
	assert.True(t, ok)

	attrs := path.AsValueMap()
	assert.False(t, attrs["pwd"].AsString() == "")
	assert.False(t, attrs["base"].AsString() == "")
}

func TestCollectBaseVariables(t *testing.T) {
	t.Setenv("GLAZE_ENV_FROM_ENV", "env-value")

	out, err := collectBaseVariables()
	assert.NoError(t, err)

	// GLAZE_ENV_* entries live under the env namespace, prefix stripped.
	assert.True(t, out["env"].GetAttr("FROM_ENV").RawEquals(cty.StringVal("env-value")))

	// the built-in path object is always present.
	_, hasPath := out["path"]
	assert.True(t, hasPath)

	// --var values are deliberately not collected here; they are resolved
	// against their variable blocks and merged under the `var` namespace by
	// VariableContext, never as bare top-level names.
	_, hasVar := out["var"]
	assert.False(t, hasVar)
}

func TestBuildEvalContext(t *testing.T) {
	vars := map[string]cty.Value{"foo": cty.StringVal("bar")}
	ctx := BuildEvalContext(vars)

	assert.Equal(t, cty.StringVal("bar"), ctx.Variables["foo"])

	// The full library, as documented in SPEC.md and README.md; a drift in
	// either direction fails here.
	expected := []string{
		"chomp", "coalesce", "concat", "csvdecode", "format", "join",
		"jsondecode", "len", "lower", "random", "regexreplace", "replace",
		"reverse", "split", "strlen", "substr", "title", "trim",
		"trimprefix", "trimspace", "trimsuffix", "upper",
	}
	assert.Len(t, ctx.Functions, len(expected))
	for _, name := range expected {
		_, ok := ctx.Functions[name]
		assert.True(t, ok, "expected function %q to be registered", name)
	}
}
