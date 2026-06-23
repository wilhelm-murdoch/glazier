package diagnostics

import (
	"errors"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
)

// a throwaway range used as the subject for the constructors that take one.
var testRange = hcl.Range{Filename: "test.glaze"}

func TestUndefinedVariableDiagnostic(t *testing.T) {
	d := UndefinedVariableDiagnostic("ghost")
	assert.Equal(t, hcl.DiagError, d.Severity)
	assert.Equal(t, "Undefined variable", d.Summary)
	assert.Contains(t, d.Detail, "ghost")
	// a flag has no source position, so the diagnostic carries no subject.
	assert.Nil(t, d.Subject)
}

func TestRequiredVariableDiagnostic(t *testing.T) {
	d := RequiredVariableDiagnostic("region", testRange)
	assert.Equal(t, hcl.DiagError, d.Severity)
	assert.Equal(t, "Missing required variable", d.Summary)
	assert.Contains(t, d.Detail, "region")
	assert.NotNil(t, d.Subject)
}

func TestInvalidVariableTypeDiagnostic(t *testing.T) {
	t.Run("names the offending keyword", func(t *testing.T) {
		d := InvalidVariableTypeDiagnostic("x", "list", testRange)
		assert.Equal(t, "Invalid variable type", d.Summary)
		assert.Contains(t, d.Detail, "list")
	})

	t.Run("handles a non-keyword type", func(t *testing.T) {
		d := InvalidVariableTypeDiagnostic("x", "", testRange)
		assert.Equal(t, "Invalid variable type", d.Summary)
		assert.Contains(t, d.Detail, "string, number or bool")
	})
}

func TestInvalidVariableDescriptionDiagnostic(t *testing.T) {
	d := InvalidVariableDescriptionDiagnostic("x", testRange)
	assert.Equal(t, "Invalid variable description", d.Summary)
	assert.Contains(t, d.Detail, "x")
}

func TestInvalidVariableDefaultDiagnostic(t *testing.T) {
	d := InvalidVariableDefaultDiagnostic("x", "number", errors.New("boom"), testRange)
	assert.Equal(t, "Invalid variable default", d.Summary)
	assert.Contains(t, d.Detail, "number")
	assert.Contains(t, d.Detail, "boom")
}

func TestInvalidVariableValueDiagnostic(t *testing.T) {
	d := InvalidVariableValueDiagnostic("x", "bool", errors.New("nope"), testRange)
	assert.Equal(t, "Invalid variable value", d.Summary)
	assert.Contains(t, d.Detail, "bool")
	assert.Contains(t, d.Detail, "nope")
}

func TestDuplicateVariableDiagnostic(t *testing.T) {
	first := hcl.Range{Filename: "test.glaze"}
	d := DuplicateVariableDiagnostic("x", first, testRange)
	assert.Equal(t, "Duplicate variable", d.Summary)
	assert.Contains(t, d.Detail, "x")
	assert.NotNil(t, d.Subject)
}
