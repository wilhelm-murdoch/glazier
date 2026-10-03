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
	unresolved, diags := p.localAttributes()
	if diags.HasErrors() {
		return nil, diags
	}

	resolved := map[string]cty.Value{}

	evalContext := func() *hcl.EvalContext {
		vars := maps.Clone(base)
		vars["local"] = cty.ObjectVal(resolved)
		return BuildEvalContext(vars)
	}

	size := 0
	for progress := true; progress && len(unresolved) > 0; {
		progress = false

		for _, name := range slices.Sorted(maps.Keys(unresolved)) {
			value, valueDiags := unresolved[name].Expr.Value(evalContext())
			if valueDiags.HasErrors() {
				continue
			}

			// Stop at the first local past the budget, before a later local can double it again.
			if size += valueSize(value, maxLocalsSize-size); size > maxLocalsSize {
				return resolved, diags.Append(localsTooLarge(name, unresolved[name].Range))
			}

			resolved[name] = value
			delete(unresolved, name)
			progress = true
		}
	}

	// What is left cannot resolve, so report the errors of each attribute, unless the pass is lenient.
	// A local in a cycle, or one that depends on a cycle, gets the cycle error in place of "Unsupported attribute".
	if requireAll {
		cycles, blocked := localCycles(unresolved)
		for _, cycle := range cycles {
			diags = diags.Append(diagnostics.LocalCycle(cycle, unresolved[cycle[0]].Range))
		}

		for _, name := range slices.Sorted(maps.Keys(unresolved)) {
			if blocked[name] {
				continue
			}

			_, valueDiags := unresolved[name].Expr.Value(evalContext())
			diags = diags.Extend(valueDiags)
		}
	}

	return resolved, diags
}

// localCycles returns each group of unresolved locals that refer to each other, sorted, and the set of locals
// that are in a group or depend on one. A local is in a cycle when it can reach itself through local references.
func localCycles(unresolved map[string]*hcl.Attribute) ([][]string, map[string]bool) {
	refs := map[string][]string{}
	for name, attr := range unresolved {
		for _, traversal := range attr.Expr.Variables() {
			if step, ok := attrStep(traversal, "local"); ok {
				if _, open := unresolved[step]; open {
					refs[name] = append(refs[name], step)
				}
			}
		}
	}

	reaches := func(from, to string) bool {
		seen := map[string]bool{}
		stack := slices.Clone(refs[from])
		for len(stack) > 0 {
			next := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if next == to {
				return true
			}

			if !seen[next] {
				seen[next] = true
				stack = append(stack, refs[next]...)
			}
		}

		return false
	}

	var cycles [][]string
	inCycle := map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(unresolved)) {
		if inCycle[name] || !reaches(name, name) {
			continue
		}

		var cycle []string
		for _, other := range slices.Sorted(maps.Keys(unresolved)) {
			if other == name || (reaches(name, other) && reaches(other, name)) {
				cycle = append(cycle, other)
				inCycle[other] = true
			}
		}

		cycles = append(cycles, cycle)
	}

	blocked := map[string]bool{}
	for name := range unresolved {
		if inCycle[name] {
			blocked[name] = true
			continue
		}

		for member := range inCycle {
			if reaches(name, member) {
				blocked[name] = true
				break
			}
		}
	}

	return cycles, blocked
}

// localAttributes returns every attribute of every `locals` block by name, and reports a name that two blocks declare.
func (p *Parser) localAttributes() (map[string]*hcl.Attribute, hcl.Diagnostics) {
	content, _, diags := p.File.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "locals"},
		},
	})

	if diags.HasErrors() {
		return nil, diags
	}

	locals := map[string]*hcl.Attribute{}

	for _, block := range content.Blocks {
		attrs, attrDiags := block.Body.JustAttributes()
		diags = diags.Extend(attrDiags)

		for name, attr := range attrs {
			if previous, ok := locals[name]; ok {
				diags = diags.Append(diagnostics.DuplicateLocal(name, previous.Range, attr.Range))
				continue
			}

			locals[name] = attr
		}
	}

	return locals, diags
}
