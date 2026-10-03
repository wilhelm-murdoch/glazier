package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

// nonDeterministicFunction is the one function that the session name must not call, because `down` computes the name again.
const nonDeterministicFunction = "random"

// DecodeSessionName evaluates only the `name` of the session block with nameSpec, for `down`.
// It gives the same name as `up`: a missing or null name is "default", and a number or a bool becomes a string.
// A variable that only windows and panes use then needs no value.
func (p *Parser) DecodeSessionName(nameSpec hcldec.Spec, ctx *hcl.EvalContext) (string, hcl.Diagnostics) {
	block, diags := p.sessionBlock()
	if diags.HasErrors() {
		return "", diags
	}

	diags = diags.Extend(p.checkSessionName(block))
	if diags.HasErrors() {
		return "", diags
	}

	decoded, _, decodeDiags := hcldec.PartialDecode(block.Body, &hcldec.ObjectSpec{"name": nameSpec}, ctx)
	if decodeDiags.HasErrors() {
		// `down` resolves only the variables it can, so name the one that the session name needs.
		if missing := p.missingNameVariables(block, ctx); missing.HasErrors() {
			return "", diags.Extend(missing)
		}

		return "", diags.Extend(decodeDiags)
	}

	diags = diags.Extend(decodeDiags)

	if name := decoded.GetAttr("name"); !name.IsNull() {
		return name.AsString(), diags
	}

	return decoders.DefaultGlazeElementName, diags
}

// checkSessionName rejects a session name that calls random(), directly or through a local.
// `down` computes the name again, so it would look for a different session than the one that `up` created.
func (p *Parser) checkSessionName(block *hcl.Block) hcl.Diagnostics {
	name, exprs := p.sessionNameExpressions(block)
	if name == nil || !callsNonDeterministic(exprs) {
		return nil
	}

	return hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "Invalid session name",
		Detail: "The session name must not use random(), directly or through a local. " +
			"`glaze down` computes the name again and would look for a different session. Use random() in a window or a pane name.",
		Subject: name.Expr.Range().Ptr(),
	}}
}

// missingNameVariables returns a "Required variable not set" error for each declared variable that the session name needs and ctx has no value for.
func (p *Parser) missingNameVariables(block *hcl.Block, ctx *hcl.EvalContext) hcl.Diagnostics {
	name, exprs := p.sessionNameExpressions(block)
	if name == nil {
		return nil
	}

	declared, _ := p.DecodeVariableBlocks()
	values := ctx.Variables["var"]

	var diags hcl.Diagnostics
	seen := map[string]bool{}
	for _, expr := range exprs {
		for _, traversal := range expr.Variables() {
			step, ok := attrStep(traversal, "var")
			if !ok || seen[step] || (values.Type().IsObjectType() && values.Type().HasAttribute(step)) {
				continue
			}

			seen[step] = true

			for _, variable := range declared {
				if variable.Name == step {
					diags = diags.Append(diagnostics.RequiredVariable(variable.Name, variable.DeclRange))
				}
			}
		}
	}

	return diags
}

// sessionNameExpressions returns the `name` attribute of the session block and every expression that it reaches:
// its own expression and the expression of each local that it refers to, through any chain of locals.
func (p *Parser) sessionNameExpressions(block *hcl.Block) (*hcl.Attribute, []hcl.Expression) {
	content, _, _ := block.Body.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "name"}},
	})

	name, ok := content.Attributes["name"]
	if !ok {
		return nil, nil
	}

	// A duplicate local is reported when the locals resolve, so this walk ignores it.
	locals, _ := p.localAttributes()

	exprs := []hcl.Expression{name.Expr}
	seen := map[string]bool{}
	for i := 0; i < len(exprs); i++ {
		for _, traversal := range exprs[i].Variables() {
			step, ok := attrStep(traversal, "local")
			if !ok || seen[step] {
				continue
			}

			seen[step] = true

			if local, ok := locals[step]; ok {
				exprs = append(exprs, local.Expr)
			}
		}
	}

	return name, exprs
}

// attrStep returns the attribute name after root in a traversal such as local.pick or var.region.
func attrStep(traversal hcl.Traversal, root string) (string, bool) {
	if traversal.RootName() != root || len(traversal) < 2 {
		return "", false
	}

	step, ok := traversal[1].(hcl.TraverseAttr)

	return step.Name, ok
}

// callsNonDeterministic reports whether one of exprs calls random().
func callsNonDeterministic(exprs []hcl.Expression) bool {
	for _, expr := range exprs {
		syntax, ok := expr.(hclsyntax.Expression)
		if !ok {
			continue
		}

		found := false
		_ = hclsyntax.VisitAll(syntax, func(node hclsyntax.Node) hcl.Diagnostics {
			if call, ok := node.(*hclsyntax.FunctionCallExpr); ok && call.Name == nonDeterministicFunction {
				found = true
			}

			return nil
		})

		if found {
			return true
		}
	}

	return false
}
