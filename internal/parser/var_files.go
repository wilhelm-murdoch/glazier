package parser

import (
	"maps"
	"os"
	"slices"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

// loadVarFile reads a --var-file and coerces each entry to its declared
// variable's type. Native HCL (`district = "x"`) files are supported.
// Entries that name an undeclared variable, or values that cannot convert
// to the declared type, are reported but never abort the load, so a run surfaces
// every var file problem at once.
func loadVarFile(path string, byName map[string]*Variable) (map[string]cty.Value, hcl.Diagnostics) {
	// The path is the user's own --var-file input to a local CLI; there is
	// no privilege boundary to traverse.
	src, err := os.ReadFile(path) //nolint:gosec // G304
	if err != nil {
		return nil, hcl.Diagnostics{diagnostics.VarFileUnreadable(path, err)}
	}

	return loadHCLVarFile(path, src, byName)
}

// loadHCLVarFile decodes a native-HCL file of `name = value` attributes.
func loadHCLVarFile(path string, src []byte, byName map[string]*Variable) (map[string]cty.Value, hcl.Diagnostics) {
	file, diags := hclparse.NewParser().ParseHCL(src, path)
	if diags.HasErrors() {
		return nil, diags
	}

	attrs, d := file.Body.JustAttributes()
	diags = diags.Extend(d)
	if diags.HasErrors() {
		return nil, diags
	}

	values := map[string]cty.Value{}
	for _, name := range slices.Sorted(maps.Keys(attrs)) {
		variable, ok := byName[name]
		if !ok {
			diags = diags.Append(diagnostics.UndeclaredVarFileVariable(name, path, attrs[name].Range))
			continue
		}

		value, valueDiags := attrs[name].Expr.Value(nil)
		diags = diags.Extend(valueDiags)
		if valueDiags.HasErrors() {
			continue
		}

		converted, err := convert.Convert(value, variable.Type)
		if err != nil {
			diags = diags.Append(diagnostics.InvalidVariableValue(name, variable.Type.FriendlyName(), err, attrs[name].Range))
			continue
		}

		values[name] = converted
	}

	return values, diags
}
