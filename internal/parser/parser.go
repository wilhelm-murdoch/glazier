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
