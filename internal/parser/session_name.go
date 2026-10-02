package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
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
	diags = diags.Extend(decodeDiags)
	if diags.HasErrors() {
		return "", diags
	}

	if name := decoded.GetAttr("name"); !name.IsNull() {
		return name.AsString(), diags
	}

	return decoders.DefaultGlazeElementName, diags
}

// checkSessionName rejects a session name that calls random(), directly or through a local.
// `down` computes the name again, so it would look for a different session than the one that `up` created.
func (p *Parser) checkSessionName(block *hcl.Block) hcl.Diagnostics {
	content, _, _ := block.Body.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "name"}},
	})

	attr, ok := content.Attributes["name"]
	if !ok {
		return nil
	}

	// A duplicate local is reported when the locals resolve, so this check ignores it.
	locals, _ := p.localAttributes()
	if !callsNonDeterministic(attr.Expr, locals, map[string]bool{}) {
		return nil
	}

	return hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "Invalid session name",
		Detail: "The session name must not use random(), directly or through a local. " +
			"`glaze down` computes the name again and would look for a different session. Use random() in a window or a pane name.",
		Subject: attr.Expr.Range().Ptr(),
	}}
}

// callsNonDeterministic reports whether expr calls random(), or refers to a local that does. seen stops a loop between locals.
func callsNonDeterministic(expr hcl.Expression, locals map[string]*hcl.Attribute, seen map[string]bool) bool {
	if syntax, ok := expr.(hclsyntax.Expression); ok {
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

	for _, traversal := range expr.Variables() {
		if traversal.RootName() != "local" || len(traversal) < 2 {
			continue
		}

		step, ok := traversal[1].(hcl.TraverseAttr)
		if !ok || seen[step.Name] {
			continue
		}
		seen[step.Name] = true

		if local, ok := locals[step.Name]; ok && callsNonDeterministic(local.Expr, locals, seen) {
			return true
		}
	}

	return false
}
