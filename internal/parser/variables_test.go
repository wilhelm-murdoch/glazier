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

	out := collectEnvVariables(envs, glazeEnvPrefix)

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

func TestCollectVariablesPrecedenceAndMerge(t *testing.T) {
	t.Setenv("GLAZE_ENV_FROM_ENV", "env-value")
	t.Setenv("GLAZE_ENV_SHARED", "env-wins-unless-flagged")

	out, err := CollectVariables([]string{
		"from_flag=flag-value",
		"SHARED=flag-overrides-env",
	})
	assert.NoError(t, err)

	// env-only variable survives the default-variable merge (regression guard
	// for the addDefaultVariables overwrite bug).
	assert.Equal(t, cty.StringVal("env-value"), out["FROM_ENV"])

	// flag-only variable survives too.
	assert.Equal(t, cty.StringVal("flag-value"), out["from_flag"])

	// flags take precedence over env for the same key.
	assert.Equal(t, cty.StringVal("flag-overrides-env"), out["SHARED"])

	// default variables are still present.
	_, hasPath := out["path"]
	assert.True(t, hasPath)
}

func TestBuildEvalContext(t *testing.T) {
	vars := map[string]cty.Value{"foo": cty.StringVal("bar")}
	ctx := BuildEvalContext(vars)

	assert.Equal(t, cty.StringVal("bar"), ctx.Variables["foo"])

	for _, name := range []string{
		"replace", "regexreplace", "upper", "lower", "reverse", "len",
		"substr", "join", "title", "trim", "trimspace", "trimsuffix",
		"trimprefix", "chomp",
	} {
		_, ok := ctx.Functions[name]
		assert.True(t, ok, "expected function %q to be registered", name)
	}
}
