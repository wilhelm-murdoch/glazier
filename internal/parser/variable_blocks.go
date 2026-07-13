package parser

import (
	"maps"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

// Variable is a declared `variable "name" {}` block. Variables give the
// otherwise free-form --var flags a self-documenting contract: a flag is
// only accepted when a matching block declares it, its value is coerced to
// the declared type, and a block without a default becomes a required input.
// Resolved variables are exposed to the rest of the profile under the `var.`
// namespace and nowhere else.
//
// The `type` attribute is optional and defaults to string, so a profile that
// only injects text can stay terse (`variable "district" {}`) while one that
// needs a number or bool can say so.
type Variable struct {
	Name        string
	Description string
	Type        cty.Type
	Default     cty.Value // cty.NilVal when the block declares no default.
	HasDefault  bool
	DeclRange   hcl.Range
}

// variableBlockSchema is the body schema of a single variable block. Using
// Content (not PartialContent) against it means any other attribute or
// nested block inside a variable declaration is rejected.
var variableBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "description"},
		{Name: "type"},
		{Name: "default"},
	},
}

// variableTypes maps the bare type keywords a variable may declare to their
// cty equivalents. Only primitives are supported; reading the keyword form
// (rather than a string) is what lets `type = string` read naturally.
var variableTypes = map[string]cty.Type{
	"string": cty.String,
	"number": cty.Number,
	"bool":   cty.Bool,
}

// DecodeVariableBlocks extracts and validates every `variable` block declared
// at the profile root. It uses PartialContent so it ignores the session block
// and its tree, letting it run as a standalone first pass before the full
// decode. Duplicate names and malformed blocks are reported but never abort
// the scan, so a single run surfaces every declaration problem at once.
func (p *Parser) DecodeVariableBlocks() ([]*Variable, hcl.Diagnostics) {
	content, _, diags := p.File.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "variable", LabelNames: []string{"name"}},
		},
	})
	if diags.HasErrors() {
		return nil, diags
	}

	var variables []*Variable
	seen := make(map[string]hcl.Range)

	for _, block := range content.Blocks {
		if block.Type != "variable" {
			continue
		}

		name := block.Labels[0]
		if previous, ok := seen[name]; ok {
			diags = diags.Append(diagnostics.DuplicateVariable(name, previous, block.DefRange))
			continue
		}
		seen[name] = block.DefRange

		variable, varDiags := decodeVariableBlock(name, block)
		diags = diags.Extend(varDiags)
		if variable != nil {
			variables = append(variables, variable)
		}
	}

	return variables, diags
}

// decodeVariableBlock validates a single variable block into a Variable. It
// returns nil (and the accumulated diagnostics) when the block is invalid,
// so callers never resolve against a half-formed declaration.
func decodeVariableBlock(name string, block *hcl.Block) (*Variable, hcl.Diagnostics) {
	attrs, diags := block.Body.Content(variableBlockSchema)
	if diags.HasErrors() {
		return nil, diags
	}

	variable := &Variable{Name: name, Type: cty.String, DeclRange: block.DefRange}
	keyword := "string"

	// type (optional): a bare keyword naming one of the supported
	// primitives, defaulting to string when omitted.
	if typeAttr, ok := attrs.Attributes["type"]; ok {
		keyword = hcl.ExprAsKeyword(typeAttr.Expr)
		declaredType, ok := variableTypes[keyword]
		if !ok {
			diags = diags.Append(diagnostics.InvalidVariableType(name, keyword, typeAttr.Expr.Range()))
			return nil, diags
		}
		variable.Type = declaredType
	}

	// description (optional): a literal string. Evaluated with a nil context
	// so it cannot reference variables or call functions; it is a static
	// label.
	if attr, ok := attrs.Attributes["description"]; ok {
		value, valueDiags := attr.Expr.Value(nil)
		diags = diags.Extend(valueDiags)
		if !valueDiags.HasErrors() {
			if value.IsNull() || value.Type() != cty.String {
				diags = diags.Append(diagnostics.InvalidVariableDescription(name, attr.Expr.Range()))
			} else {
				variable.Description = value.AsString()
			}
		}
	}

	// default (optional): a literal value converted to the declared type, so
	// `default = 1` satisfies a number without the author quoting it.
	if attr, ok := attrs.Attributes["default"]; ok {
		value, valueDiags := attr.Expr.Value(nil)
		diags = diags.Extend(valueDiags)
		if !valueDiags.HasErrors() {
			converted, err := convert.Convert(value, variable.Type)
			if err != nil {
				diags = diags.Append(diagnostics.InvalidVariableDefault(name, keyword, err, attr.Expr.Range()))
			} else {
				variable.Default = converted
				variable.HasDefault = true
			}
		}
	}

	if diags.HasErrors() {
		return nil, diags
	}

	return variable, diags
}

// collectFlagVariables parses variables passed via repeated --var flags.
// Later entries win, so a flag can override an earlier one for the same name.
func collectFlagVariables(vars []string) map[string]cty.Value {
	out := make(map[string]cty.Value)

	for _, flag := range vars {
		key, value, ok := strings.Cut(flag, "=")
		if !ok {
			continue
		}

		out[strings.TrimSpace(key)] = cty.StringVal(value)
	}

	return out
}

// ResolveVariables turns the declared variables, an optional --var-file, and
// the raw --var flags into the concrete `var.*` value map. Precedence is,
// last write wins: declared defaults, then the var-file, then the flags. Each
// supplied value is coerced to the variable's declared type. A flag or
// var-file entry naming an undeclared variable is always an error; a declared
// variable left with neither value nor default is reported as required only
// when requireAll is set. requireAll is false for `down`, which evaluates
// only the session name.
func ResolveVariables(declared []*Variable, flags []string, varFile string, requireAll bool) (map[string]cty.Value, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	byName := make(map[string]*Variable, len(declared))
	out := make(map[string]cty.Value, len(declared))
	for _, variable := range declared {
		byName[variable.Name] = variable
		if variable.HasDefault {
			out[variable.Name] = variable.Default
		}
	}

	if varFile != "" {
		fileValues, fileDiags := loadVarFile(varFile, byName)
		diags = diags.Extend(fileDiags)
		maps.Copy(out, fileValues)
	}

	for name, raw := range collectFlagVariables(flags) {
		variable, ok := byName[name]
		if !ok {
			diags = diags.Append(diagnostics.UndefinedVariable(name))
			continue
		}

		value, err := convert.Convert(raw, variable.Type)
		if err != nil {
			diags = diags.Append(diagnostics.InvalidVariableValue(
				name, variable.Type.FriendlyName(), err, variable.DeclRange,
			))
			continue
		}

		out[name] = value
	}

	if requireAll {
		for _, variable := range declared {
			if _, ok := out[variable.Name]; !ok {
				diags = diags.Append(diagnostics.RequiredVariable(variable.Name, variable.DeclRange))
			}
		}
	}

	return out, diags
}
