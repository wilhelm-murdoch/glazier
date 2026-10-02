package parser

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const (
	// maxNestingDepth is how deep brackets, braces, parentheses and strings may nest in a file.
	// The parser recurses once for each level, and about 100,000 levels overflow the Go stack, which no recover can catch.
	maxNestingDepth = 256

	// maxLocalsSize is the budget for all resolved locals together: the bytes of each string plus one for each element.
	// A chain of locals that each double the one before reaches gigabytes in a few lines.
	maxLocalsSize = 1 << 20
)

// checkNesting rejects src when it nests deeper than maxNestingDepth.
// It reads the tokens of the lexer, which does not recurse, so a deep file cannot crash it.
func checkNesting(src []byte, filename string) hcl.Diagnostics {
	tokens, _ := hclsyntax.LexConfig(src, filename, hcl.InitialPos)

	depth := 0
	for _, token := range tokens {
		switch token.Type {
		case hclsyntax.TokenOBrace, hclsyntax.TokenOBrack, hclsyntax.TokenOParen, hclsyntax.TokenOQuote,
			hclsyntax.TokenOHeredoc, hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
			depth++
			if depth > maxNestingDepth {
				return hcl.Diagnostics{{
					Severity: hcl.DiagError,
					Summary:  "Nesting too deep",
					Detail:   fmt.Sprintf("Glaze accepts at most %d levels of brackets, braces, parentheses and strings inside each other.", maxNestingDepth),
					Subject:  token.Range.Ptr(),
				}}
			}
		case hclsyntax.TokenCBrace, hclsyntax.TokenCBrack, hclsyntax.TokenCParen, hclsyntax.TokenCQuote,
			hclsyntax.TokenCHeredoc, hclsyntax.TokenTemplateSeqEnd:
			depth = max(depth-1, 0)
		}
	}

	return nil
}

// valueSize returns the size of value for the locals budget, and stops counting once it passes limit.
// A value can repeat a large value many times without new memory, so the count must stop early to stay fast.
func valueSize(value cty.Value, limit int) int {
	if value.IsNull() || !value.IsKnown() {
		return 0
	}

	if value.Type() == cty.String {
		return len(value.AsString())
	}

	if !value.CanIterateElements() {
		return 1
	}

	size := 0
	for it := value.ElementIterator(); it.Next() && size <= limit; {
		_, element := it.Element()
		size += 1 + valueSize(element, limit-size)
	}

	return size
}

// localsTooLarge returns the error for the local whose value takes all locals past maxLocalsSize.
func localsTooLarge(name string, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Locals too large",
		Detail: fmt.Sprintf(
			"The value of local.%s takes all locals together past %d KiB. Glaze limits locals, because each local can double the size of the one before.",
			name, maxLocalsSize/1024,
		),
		Subject: subject.Ptr(),
	}
}
