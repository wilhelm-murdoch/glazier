package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// writeVarFile writes contents to a temp file with the given name and returns
// its full path.
func writeVarFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func TestLoadVarFile(t *testing.T) {
	byName := map[string]*Variable{
		"district": {Name: "district", Type: cty.String},
		"count":    {Name: "count", Type: cty.Number},
		"on":       {Name: "on", Type: cty.Bool},
	}

	t.Run("loads an HCL file and coerces to the declared types", func(t *testing.T) {
		path := writeVarFile(t, "vars.glazevars", `
district = "watson"
count    = 3
on       = true
`)
		values, diags := loadVarFile(path, byName)
		assert.False(t, diags.HasErrors())
		assert.True(t, values["district"].RawEquals(cty.StringVal("watson")))
		assert.True(t, values["count"].RawEquals(cty.NumberIntVal(3)))
		assert.True(t, values["on"].RawEquals(cty.True))
	})

	t.Run("reports an unreadable file", func(t *testing.T) {
		_, diags := loadVarFile(filepath.Join(t.TempDir(), "missing.glazevars"), byName)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Unable to read var file")
	})

	t.Run("reports unparsable HCL", func(t *testing.T) {
		path := writeVarFile(t, "vars.glazevars", `district = `)
		_, diags := loadVarFile(path, byName)
		assert.True(t, diags.HasErrors())
	})

	t.Run("an undeclared HCL entry is an error but does not abort the load", func(t *testing.T) {
		path := writeVarFile(t, "vars.glazevars", `
district = "watson"
ghost    = "boo"
`)
		values, diags := loadVarFile(path, byName)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Undefined variable")
		assert.True(t, values["district"].RawEquals(cty.StringVal("watson")))
	})

	t.Run("a value that cannot convert is an error but does not abort the load", func(t *testing.T) {
		path := writeVarFile(t, "vars.glazevars", `
count    = "notanumber"
district = "watson"
`)
		values, diags := loadVarFile(path, byName)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Invalid variable value")
		assert.True(t, values["district"].RawEquals(cty.StringVal("watson")))
		_, ok := values["count"]
		assert.False(t, ok)
	})
}

// TestResolveVariablesWithVarFile covers the precedence contract between
// declared defaults, the --var-file, and the --var flags.
func TestResolveVariablesWithVarFile(t *testing.T) {
	declared := []*Variable{{
		Name: "district", Type: cty.String,
		Default: cty.StringVal("default"), HasDefault: true,
	}}

	t.Run("a var-file value overrides a default", func(t *testing.T) {
		path := writeVarFile(t, "vars.glazevars", `district = "from-file"`)
		out, diags := ResolveVariables(declared, nil, path, true)
		assert.False(t, diags.HasErrors())
		assert.True(t, out["district"].RawEquals(cty.StringVal("from-file")))
	})

	t.Run("a --var flag overrides a var-file value", func(t *testing.T) {
		path := writeVarFile(t, "vars.glazevars", `district = "from-file"`)
		out, diags := ResolveVariables(declared, []string{"district=from-flag"}, path, true)
		assert.False(t, diags.HasErrors())
		assert.True(t, out["district"].RawEquals(cty.StringVal("from-flag")))
	})

	t.Run("a var-file value satisfies a required variable", func(t *testing.T) {
		required := []*Variable{{Name: "district", Type: cty.String}}
		path := writeVarFile(t, "vars.glazevars", `district = "from-file"`)
		out, diags := ResolveVariables(required, nil, path, true)
		assert.False(t, diags.HasErrors())
		assert.True(t, out["district"].RawEquals(cty.StringVal("from-file")))
	})

	t.Run("var-file diagnostics surface through resolution", func(t *testing.T) {
		_, diags := ResolveVariables(declared, nil, filepath.Join(t.TempDir(), "missing"), true)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags.Error(), "Unable to read var file")
	})
}
