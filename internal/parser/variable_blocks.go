package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

// Variable is a declared `variable "name" {}` block. Variables give the
// otherwise free-form --var flags a typed, self-documenting contract: a flag is
// only accepted when a matching block declares it, its value is coerced to the
// declared type, and a block without a default becomes a required input.
// Resolved variables are exposed to the rest of the profile under the `var.`
// namespace and nowhere else.
type Variable struct {
	Name        string
	Description string
	Type        cty.Type
	Default     cty.Value // cty.NilVal when the block declares no default.
	HasDefault  bool
	DeclRange   hcl.Range
}

// variableBlockSchema is the body schema of a single variable block. Using
// Content (not PartialContent) against it means any other attribute or nested
// block inside a variable declaration is rejected.
var variableBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "description"},
		{Name: "type", Required: true},
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
// decode. Duplicate names and malformed blocks are reported but never abort the
// scan, so a single run surfaces every declaration problem at once.
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
			diags = diags.Append(diagnostics.DuplicateVariableDiagnostic(name, previous, block.DefRange))
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
// returns nil (and the accumulated diagnostics) when the block is invalid, so
// callers never resolve against a half-formed declaration.
func decodeVariableBlock(name string, block *hcl.Block) (*Variable, hcl.Diagnostics) {
	attrs, diags := block.Body.Content(variableBlockSchema)
	if diags.HasErrors() {
		return nil, diags
	}

	variable := &Variable{Name: name, DeclRange: block.DefRange}

	// type (required): a bare keyword naming one of the supported primitives.
	typeAttr := attrs.Attributes["type"]
	keyword := hcl.ExprAsKeyword(typeAttr.Expr)
	declaredType, ok := variableTypes[keyword]
	if !ok {
		diags = diags.Append(diagnostics.InvalidVariableTypeDiagnostic(name, keyword, typeAttr.Expr.Range()))
		return nil, diags
	}
	variable.Type = declaredType

	// description (optional): a literal string. Evaluated with a nil context so
	// it cannot reference variables or call functions; it is a static label.
	if attr, ok := attrs.Attributes["description"]; ok {
		value, valueDiags := attr.Expr.Value(nil)
		diags = diags.Extend(valueDiags)
		if !valueDiags.HasErrors() {
			if value.IsNull() || value.Type() != cty.String {
				diags = diags.Append(diagnostics.InvalidVariableDescriptionDiagnostic(name, attr.Expr.Range()))
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
			converted, err := convert.Convert(value, declaredType)
			if err != nil {
				diags = diags.Append(diagnostics.InvalidVariableDefaultDiagnostic(name, keyword, err, attr.Expr.Range()))
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

// ResolveVariables turns the declared variables and the raw --var flags into
// the concrete `var.*` value map. Each declared variable takes its flag value
// (coerced to the declared type), then its default; a flag naming an undeclared
// variable is always an error. A declared variable with neither flag nor
// default is reported as required only when requireAll is set. requireAll is
// false for `down`, which evaluates only the session name and must not demand
// variables used solely deeper in the profile.
func ResolveVariables(declared []*Variable, flags []string, requireAll bool) (map[string]cty.Value, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	byName := make(map[string]*Variable, len(declared))
	for _, variable := range declared {
		byName[variable.Name] = variable
	}

	supplied := collectFlagVariables(flags)
	for name := range supplied {
		if _, ok := byName[name]; !ok {
			diags = diags.Append(diagnostics.UndefinedVariableDiagnostic(name))
		}
	}

	out := make(map[string]cty.Value, len(declared))
	for _, variable := range declared {
		if raw, ok := supplied[variable.Name]; ok {
			value, err := convert.Convert(raw, variable.Type)
			if err != nil {
				diags = diags.Append(diagnostics.InvalidVariableValueDiagnostic(
					variable.Name, variable.Type.FriendlyName(), err, variable.DeclRange,
				))
				continue
			}
			out[variable.Name] = value
			continue
		}

		if variable.HasDefault {
			out[variable.Name] = variable.Default
			continue
		}

		if requireAll {
			diags = diags.Append(diagnostics.RequiredVariableDiagnostic(variable.Name, variable.DeclRange))
		}
	}

	return out, diags
}

// VariableContext builds the evaluation context for a profile: the built-in
// top-level variables (GLAZE_ENV_* and the path object) plus the declared
// `var` object resolved from the --var flags. requireAll enforces that every
// declared variable without a default has been supplied; `down` passes false
// because it evaluates only the session name. The returned context is always
// usable even when diagnostics contain errors, so callers can render the full
// set before deciding to halt.
func (p *Parser) VariableContext(flags []string, requireAll bool) (*hcl.EvalContext, hcl.Diagnostics) {
	base, err := collectBaseVariables()
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Could not collect variables",
			Detail:   err.Error(),
		}}
	}

	declared, diags := p.DecodeVariableBlocks()

	resolved, resolveDiags := ResolveVariables(declared, flags, requireAll)
	diags = diags.Extend(resolveDiags)

	base["var"] = cty.ObjectVal(resolved)

	return BuildEvalContext(base), diags
}
