package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclparse"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
)

// topLevelSchema allows one session block and any variable and locals blocks at the root, and rejects anything else.
var topLevelSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "session"},
		{Type: "variable", LabelNames: []string{"name"}},
		{Type: "locals"},
	},
}

// Parser holds a parsed profile.
type Parser struct {
	File *hcl.File
}

// New parses the profile at path.
func New(path string) (*Parser, hcl.Diagnostics) {
	return newParser(hclparse.NewParser().ParseHCLFile(path))
}

// NewFromBytes parses a profile in memory, for the fuzz tests. The filename only labels the diagnostics.
func NewFromBytes(src []byte, filename string) (*Parser, hcl.Diagnostics) {
	return newParser(hclparse.NewParser().ParseHCL(src, filename))
}

// newParser returns a parser for file, or the diagnostics when the file has syntax errors.
func newParser(file *hcl.File, diags hcl.Diagnostics) (*Parser, hcl.Diagnostics) {
	if diags.HasErrors() {
		return nil, diags
	}

	return &Parser{File: file}, nil
}

// missingSession returns the error for a profile without a session block.
func (p *Parser) missingSession() *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Missing session block",
		Detail:   "A block of type \"session\" is required here.",
		Subject:  p.File.Body.MissingItemRange().Ptr(),
	}
}

// DecodeSessionName evaluates only the `name` of the session block, for `down`.
// A variable that only windows and panes use then needs no value.
func (p *Parser) DecodeSessionName(ctx *hcl.EvalContext) (string, hcl.Diagnostics) {
	content, _, diags := p.File.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "session"}},
	})
	if diags.HasErrors() {
		return "", diags
	}

	if len(content.Blocks) == 0 {
		return "", append(diags, p.missingSession())
	}

	// The schema has only the session block, so the first block is the session.
	attrs, _, attrDiags := content.Blocks[0].Body.PartialContent(&hcl.BodySchema{
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

// sessionBlock returns the single session block, so that variable and locals blocks can sit beside it.
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
		return nil, hcl.Diagnostics{p.missingSession()}
	}

	return session, nil
}

// Decode decodes the session block with bodySpec, the spec for the body of the block (see spec.Session).
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

	// Return the warnings from a successful decode, so callers can show them.
	return decoders.NewSession(decodedSpec), diags
}
