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

// topLevelSchema describes everything allowed at the root of a profile: the
// single session block and any number of variable declarations. Decoding the
// root against this exact schema keeps the parser strict (a stray top-level
// attribute or misspelled block is still an error) while letting `variable`
// blocks sit alongside the session.
var topLevelSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "session"},
		{Type: "variable", LabelNames: []string{"name"}},
	},
}

// sessionBlock extracts the single required session block from the profile
// root. Pulling it out by hand (rather than letting hcldec decode the whole
// file) is what lets sibling `variable` blocks coexist with the session: they
// are declared in the schema and simply ignored here.
func (p *Parser) sessionBlock() (*hcl.Block, hcl.Diagnostics) {
	content, diags := p.File.Body.Content(topLevelSchema)
	if diags.HasErrors() {
		return nil, diags
	}

	var session *hcl.Block
	for _, block := range content.Blocks {
		if block.Type != "session" {
			continue
		}

		if session != nil {
			return nil, hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "Duplicate session block",
				Detail:   "A profile may define only one session block.",
				Subject:  block.DefRange.Ptr(),
			}}
		}

		session = block
	}

	if session == nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Missing session block",
			Detail:   "A block of type \"session\" is required here.",
			Subject:  p.File.Body.MissingItemRange().Ptr(),
		}}
	}

	return session, nil
}

// Decode is responsible for decoding the HCL file into a session.Session
// struct. The bodySpec describes the session block's body (see spec.Session);
// the session block itself is located here so that any top-level `variable`
// declarations are tolerated rather than rejected as unexpected blocks.
func (p *Parser) Decode(
	bodySpec hcldec.Spec,
	ctx *hcl.EvalContext,
) (*decoders.Session, hcl.Diagnostics) {
	block, diags := p.sessionBlock()
	if diags.HasErrors() {
		return nil, diags
	}

	decodedSpec, decodeDiags := hcldec.Decode(block.Body, bodySpec, ctx)
	diags = diags.Extend(decodeDiags)
	if diags.HasErrors() {
		return nil, diags
	}

	if decodedSpec.IsNull() {
		// We should never get to this point: sessionBlock guarantees a session
		// block exists, and decoding its body yields a non-null object.
		panic("glaze definition invalid")
	}

	session := decoders.NewSession(decodedSpec)
	if sessionDiags := session.Decode(); sessionDiags.HasErrors() {
		return nil, sessionDiags
	}

	return session, nil
}
