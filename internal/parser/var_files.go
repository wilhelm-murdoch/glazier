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

// loadVarFile reads an HCL --var-file of `name = value` lines and converts each value to the type of its variable.
// It reports every undeclared name and every bad value, and does not stop at the first one.
func loadVarFile(path string, byName map[string]*Variable) (map[string]cty.Value, hcl.Diagnostics) {
	// The path is the user's own --var-file input to a local CLI; there is
	// no privilege boundary to traverse.
	src, err := os.ReadFile(path) //nolint:gosec // G304
	if err != nil {
		return nil, hcl.Diagnostics{diagnostics.VarFileUnreadable(path, err)}
	}

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
