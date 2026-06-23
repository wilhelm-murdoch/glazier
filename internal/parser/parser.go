package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclparse"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
)

type Parser struct {
	File   *hcl.File
	parser *hclparse.Parser
}

// New is responsible for creating a new Parser and parsing the specified HCL file.
func New(path string) (*Parser, hcl.Diagnostics) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCLFile(path)

	if diags.HasErrors() {
		return nil, diags
	}

	return &Parser{
		File:   file,
		parser: parser,
	}, nil
}

// NewFromBytes parses an in-memory HCL profile. The filename only labels
// diagnostics; nothing is read from disk. This is the entry point the fuzz
// tests use, which would otherwise have to write a file per generated input.
func NewFromBytes(src []byte, filename string) (*Parser, hcl.Diagnostics) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, filename)

	if diags.HasErrors() {
		return nil, diags
	}

	return &Parser{
		File:   file,
		parser: parser,
	}, nil
}

// DecodeSessionName evaluates only the session block's `name` attribute. Unlike
// Decode it never touches the window/pane tree, so a profile that interpolates
// variables deeper down (e.g. inside a pane command) can still be torn down by
// `glaze down` without supplying every one of those variables. Only the
// variables referenced by `name` itself must resolve. This keeps interpolated
// session names working while sparing `down` the full evaluation that `up` needs.
func (p *Parser) DecodeSessionName(ctx *hcl.EvalContext) (string, hcl.Diagnostics) {
	content, _, diags := p.File.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "session"}},
	})
	if diags.HasErrors() {
		return "", diags
	}

	for _, block := range content.Blocks {
		if block.Type != "session" {
			continue
		}

		attrs, _, attrDiags := block.Body.PartialContent(&hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{{Name: "name", Required: true}},
		})
		diags = append(diags, attrDiags...)
		if diags.HasErrors() {
			return "", diags
		}

		value, valueDiags := attrs.Attributes["name"].Expr.Value(ctx)
		diags = append(diags, valueDiags...)
		if diags.HasErrors() {
			return "", diags
		}

		return value.AsString(), diags
	}

	return "", append(diags, &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Missing session block",
		Detail:   "A block of type \"session\" is required here.",
		Subject:  p.File.Body.MissingItemRange().Ptr(),
	})
}

// Decode is responsible for decoding the HCL file into a session.Session struct.
func (p *Parser) Decode(
	spec hcldec.Spec,
	ctx *hcl.EvalContext,
) (*decoders.Session, hcl.Diagnostics) {
	decodedSpec, diags := hcldec.Decode(p.File.Body, spec, ctx)
	if diags.HasErrors() {
		return nil, diags
	}

	if decodedSpec.IsNull() {
		// We should never get to this point as the HCL specification for a glaze
		// definition file would return a validation error if no session block is
		// defined.
		panic("glaze definition invalid")
	}

	session := decoders.NewSession(decodedSpec)
	if diags := session.Decode(); diags.HasErrors() {
		return nil, diags
	}

	return session, nil
}
