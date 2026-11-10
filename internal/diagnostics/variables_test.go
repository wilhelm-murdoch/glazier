package diagnostics

import (
	"errors"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
)

// a throwaway range used as the subject for the constructors that take one.
var testRange = hcl.Range{Filename: "test.glaze"}

func TestVariableDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		diag    *hcl.Diagnostic
		summary string
		detail  []string
		subject bool
	}{
		{"an undefined variable from a flag has no subject", UndefinedVariable("ghost"), "Undefined variable", []string{"ghost"}, false}, // a flag has no source position
		{"a required variable has a subject", RequiredVariable("region", testRange), "Required variable not set", []string{"region"}, true},
		{"an invalid type names the keyword", InvalidVariableType("x", "list", testRange), "Invalid variable type", []string{"list"}, true},
		{"an invalid type without a keyword lists the types", InvalidVariableType("x", "", testRange), "Invalid variable type", []string{"string, number or bool"}, true},
		{"an invalid description names the variable", InvalidVariableDescription("x", testRange), "Invalid variable description", []string{"x"}, true},
		{"an invalid default names the type and the cause", InvalidVariableDefault("x", "number", errors.New("boom"), testRange), "Invalid variable default", []string{"number", "boom"}, true},
		{"an invalid value names the type and the cause", InvalidVariableValue("x", "bool", errors.New("nope"), testRange), "Invalid variable value", []string{"bool", "nope"}, true},
		{"a duplicate variable names the variable", DuplicateVariable("x", testRange, testRange), "Duplicate variable declaration", []string{"x"}, true},
		{"a duplicate local names the local", DuplicateLocal("accent", testRange, testRange), "Duplicate local value", []string{"accent"}, true},
		{"an unreadable var file has no subject", VarFileUnreadable("vars.json", errors.New("permission denied")), "Unable to read var file", []string{"vars.json", "permission denied"}, false},
		{"an undeclared var file variable names the file", UndeclaredVarFileVariable("ghost", "vars.glazevars", testRange), "Undefined variable", []string{"ghost", "vars.glazevars"}, true},
		{"an undeclared var file variable from JSON has no subject", UndeclaredVarFileVariable("ghost", "vars.json", hcl.Range{}), "Undefined variable", []string{"ghost"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, hcl.DiagError, tc.diag.Severity)
			assert.Equal(t, tc.summary, tc.diag.Summary)

			for _, part := range tc.detail {
				assert.Contains(t, tc.diag.Detail, part)
			}

			assert.Equal(t, tc.subject, tc.diag.Subject != nil)
		})
	}
}
