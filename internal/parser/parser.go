package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclparse"

	"github.com/wilhelm-murdoch/glazier/internal/schema/session"
)

type Parser struct {
	File   *hcl.File
	parser *hclparse.Parser
}

// Decode is responsible for decoding the HCL file into a session.Session struct.
func (p *Parser) Decode(
	spec hcldec.Spec,
	ctx *hcl.EvalContext,
) (*session.Session, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	decoded, diags := hcldec.Decode(p.File.Body, spec, ctx)
	if diags.HasErrors() {
		return nil, diags
	}

	session := new(session.Session)

	it := decoded.ElementIterator()
	for it.Next() {
		_, value := it.Element()
		if diagsDecode := session.Decode(value); diagsDecode.HasErrors() {
			diags = diags.Extend(diagsDecode)
			continue
		}
	}

	return session, diags
}

// NewParser is responsible for creating a new Parser instance and parsing the HCL file.
func NewParser(path string) (*Parser, hcl.Diagnostics) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCLFile(path)

	if diags.HasErrors() {
		return nil, diags
	}

	return &Parser{
		parser: parser,
		File:   file,
	}, nil
}
