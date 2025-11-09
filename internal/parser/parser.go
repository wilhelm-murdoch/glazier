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
	decodedGlazeDefinition, diags := hcldec.Decode(p.File.Body, spec, ctx)
	if diags.HasErrors() {
		return nil, diags
	}

	if decodedGlazeDefinition.IsNull() {
		// We should never get to this point as the HCL specification for a glaze
		// definition file would return a validation error if no session block is
		// defined.
		panic("glaze definition invalid")
	}

	session, _ := session.New()
	if diags := session.Decode(decodedGlazeDefinition); diags.HasErrors() {
		return nil, diags
	}

	return session, nil
}

// New is responsible for creating a new Parser and parsing the specified HCL file.
func New(path string) (*Parser, hcl.Diagnostics) {
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
