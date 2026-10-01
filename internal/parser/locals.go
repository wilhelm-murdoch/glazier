package parser

import (
	"maps"
	"slices"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

// resolveLocals evaluates all `locals` blocks, which can refer to env, path, var, the functions and each other in any order.
// It repeats passes until one makes no progress, then reports the real errors. With requireAll false, it drops unresolved locals.
func (p *Parser) resolveLocals(base map[string]cty.Value, requireAll bool) (map[string]cty.Value, hcl.Diagnostics) {
	content, _, diags := p.File.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "locals"},
		},
	})
	if diags.HasErrors() {
		return nil, diags
	}

	unresolved := map[string]*hcl.Attribute{}

	for _, block := range content.Blocks {
		attrs, attrDiags := block.Body.JustAttributes()
		diags = diags.Extend(attrDiags)

		for name, attr := range attrs {
			if previous, ok := unresolved[name]; ok {
				diags = diags.Append(diagnostics.DuplicateLocal(name, previous.Range, attr.Range))
				continue
			}
			unresolved[name] = attr
		}
	}

	resolved := map[string]cty.Value{}

	evalContext := func() *hcl.EvalContext {
		vars := maps.Clone(base)
		vars["local"] = cty.ObjectVal(resolved)
		return BuildEvalContext(vars)
	}

	for progress := true; progress && len(unresolved) > 0; {
		progress = false

		for _, name := range slices.Sorted(maps.Keys(unresolved)) {
			value, valueDiags := unresolved[name].Expr.Value(evalContext())
			if valueDiags.HasErrors() {
				continue
			}

			resolved[name] = value
			delete(unresolved, name)
			progress = true
		}
	}

	// What is left cannot resolve, so report the errors of each attribute, unless the pass is lenient.
	if requireAll {
		for _, name := range slices.Sorted(maps.Keys(unresolved)) {
			_, valueDiags := unresolved[name].Expr.Value(evalContext())
			diags = diags.Extend(valueDiags)
		}
	}

	return resolved, diags
}
