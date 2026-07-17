package parser

import (
	"maps"
	"slices"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

// resolveLocals evaluates every `locals { name = expr }` block at the
// profile root into the `local.*` value map. Locals may reference env.*,
// path.*, var.*, the function library, and each other (in any declaration
// order): resolution iterates, evaluating whatever it can each pass, until a
// pass makes no progress. Whatever still fails then reports its real
// evaluation diagnostics, so a genuine error (a typo'd var, a bad function
// call) is never masked by the ordering machinery.
//
// requireAll mirrors the variable-resolution flag: when false (`down`, which
// only needs the session name) locals that cannot resolve are dropped
// silently rather than reported, so a broken local nobody references does not
// block the teardown.
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

	// Whatever is left is genuinely unresolvable: surface each attribute's
	// own evaluation diagnostics. A lenient pass skips this; an unresolved
	// local simply never appears in the returned map.
	if requireAll {
		for _, name := range slices.Sorted(maps.Keys(unresolved)) {
			_, valueDiags := unresolved[name].Expr.Value(evalContext())
			diags = diags.Extend(valueDiags)
		}
	}

	return resolved, diags
}
