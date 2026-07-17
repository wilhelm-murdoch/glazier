package diagnostics

import (
	"errors"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
)

// a throwaway range used as the subject for the constructors that take one.
var testRange = hcl.Range{Filename: "test.glaze"}

func TestUndefinedVariable(t *testing.T) {
	d := UndefinedVariable("ghost")
	assert.Equal(t, hcl.DiagError, d.Severity)
	assert.Equal(t, "Undefined variable", d.Summary)
	assert.Contains(t, d.Detail, "ghost")
	// a flag has no source position, so the diagnostic carries no subject.
	assert.Nil(t, d.Subject)
}

func TestRequiredVariable(t *testing.T) {
	d := RequiredVariable("region", testRange)
	assert.Equal(t, hcl.DiagError, d.Severity)
	assert.Equal(t, "Required variable not set", d.Summary)
	assert.Contains(t, d.Detail, "region")
	assert.NotNil(t, d.Subject)
}

func TestInvalidVariableType(t *testing.T) {
	t.Run("names the offending keyword", func(t *testing.T) {
		d := InvalidVariableType("x", "list", testRange)
		assert.Equal(t, "Invalid variable type", d.Summary)
		assert.Contains(t, d.Detail, "list")
	})

	t.Run("handles a non-keyword type", func(t *testing.T) {
		d := InvalidVariableType("x", "", testRange)
		assert.Equal(t, "Invalid variable type", d.Summary)
		assert.Contains(t, d.Detail, "string, number or bool")
	})
}

func TestInvalidVariableDescription(t *testing.T) {
	d := InvalidVariableDescription("x", testRange)
	assert.Equal(t, "Invalid variable description", d.Summary)
	assert.Contains(t, d.Detail, "x")
}

func TestInvalidVariableDefault(t *testing.T) {
	d := InvalidVariableDefault("x", "number", errors.New("boom"), testRange)
	assert.Equal(t, "Invalid variable default", d.Summary)
	assert.Contains(t, d.Detail, "number")
	assert.Contains(t, d.Detail, "boom")
}

func TestInvalidVariableValue(t *testing.T) {
	d := InvalidVariableValue("x", "bool", errors.New("nope"), testRange)
	assert.Equal(t, "Invalid variable value", d.Summary)
	assert.Contains(t, d.Detail, "bool")
	assert.Contains(t, d.Detail, "nope")
}

func TestDuplicateVariable(t *testing.T) {
	first := hcl.Range{Filename: "test.glaze"}
	d := DuplicateVariable("x", first, testRange)
	assert.Equal(t, "Duplicate variable declaration", d.Summary)
	assert.Contains(t, d.Detail, "x")
	assert.NotNil(t, d.Subject)
}

func TestDuplicateLocal(t *testing.T) {
	first := hcl.Range{Filename: "test.glaze"}
	d := DuplicateLocal("accent", first, testRange)
	assert.Equal(t, "Duplicate local value", d.Summary)
	assert.Contains(t, d.Detail, "accent")
	assert.NotNil(t, d.Subject)
}

func TestVarFileUnreadable(t *testing.T) {
	d := VarFileUnreadable("vars.json", errors.New("permission denied"))
	assert.Equal(t, hcl.DiagError, d.Severity)
	assert.Equal(t, "Unable to read var file", d.Summary)
	assert.Contains(t, d.Detail, "vars.json")
	assert.Contains(t, d.Detail, "permission denied")
}

func TestVarFileInvalid(t *testing.T) {
	d := VarFileInvalid("vars.json", "unexpected end of input")
	assert.Equal(t, hcl.DiagError, d.Severity)
	assert.Equal(t, "Invalid var file", d.Summary)
	assert.Contains(t, d.Detail, "vars.json")
	assert.Contains(t, d.Detail, "unexpected end of input")
}

func TestUndeclaredVarFileVariable(t *testing.T) {
	t.Run("carries a subject when the entry has a range", func(t *testing.T) {
		d := UndeclaredVarFileVariable("ghost", "vars.glazevars", testRange)
		assert.Equal(t, "Undefined variable", d.Summary)
		assert.Contains(t, d.Detail, "ghost")
		assert.Contains(t, d.Detail, "vars.glazevars")
		assert.NotNil(t, d.Subject)
	})

	t.Run("omits the subject for rangeless JSON entries", func(t *testing.T) {
		d := UndeclaredVarFileVariable("ghost", "vars.json", hcl.Range{})
		assert.Nil(t, d.Subject)
	})
}
