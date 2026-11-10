package diagnostics

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

// This file holds the diagnostics for variable and locals blocks, --var flags and --var-file.

// errorAt returns an error diagnostic. It sets the subject only when subject names a file, because a flag has no place.
func errorAt(summary, detail string, subject hcl.Range) *hcl.Diagnostic {
	diag := &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  summary,
		Detail:   detail,
	}

	if subject.Filename != "" {
		diag.Subject = subject.Ptr()
	}

	return diag
}

// DuplicateVariable flags two variable blocks sharing a name.
func DuplicateVariable(name string, previous, subject hcl.Range) *hcl.Diagnostic {
	return errorAt("Duplicate variable declaration", fmt.Sprintf("A variable named %q was already declared at %s.", name, previous.String()), subject)
}

// InvalidVariableType flags an unsupported variable type. keyword is the bad keyword, or "" when `type` is not a keyword.
func InvalidVariableType(name, keyword string, subject hcl.Range) *hcl.Diagnostic {
	detail := fmt.Sprintf("Variable %q must declare its type as one of the bare keywords string, number or bool.", name)
	if keyword != "" {
		detail = fmt.Sprintf("Variable %q declares unsupported type %q. Supported types are: string, number, bool.", name, keyword)
	}

	return errorAt("Invalid variable type", detail, subject)
}

// InvalidVariableDescription flags a variable description that is not a literal string.
func InvalidVariableDescription(name string, subject hcl.Range) *hcl.Diagnostic {
	return errorAt("Invalid variable description", fmt.Sprintf("The description of variable %q must be a literal string.", name), subject)
}

// InvalidVariableDefault flags a default that cannot convert to the declared type.
func InvalidVariableDefault(name, keyword string, err error, subject hcl.Range) *hcl.Diagnostic {
	return errorAt("Invalid variable default", fmt.Sprintf("The default for variable %q is not a valid %s: %s.", name, keyword, err), subject)
}

// UndefinedVariable flags a --var flag naming a variable no block declares.
func UndefinedVariable(name string) *hcl.Diagnostic {
	return errorAt("Undefined variable", fmt.Sprintf("A value was supplied for variable %q, but the profile declares no such variable block.", name), hcl.Range{})
}

// InvalidVariableValue flags a supplied value that cannot convert to the declared type.
func InvalidVariableValue(name, friendlyType string, err error, subject hcl.Range) *hcl.Diagnostic {
	return errorAt("Invalid variable value", fmt.Sprintf("The value supplied for variable %q is not a valid %s: %s.", name, friendlyType, err), subject)
}

// RequiredVariable flags a declared variable with neither a supplied value nor a default.
func RequiredVariable(name string, subject hcl.Range) *hcl.Diagnostic {
	return errorAt("Required variable not set", fmt.Sprintf("Variable %q declares no default, so a value must be supplied with --var %s=... or via --var-file.", name, name), subject)
}

// LocalCycle flags locals that refer to each other, so that none of them can have a value.
func LocalCycle(names []string, subject hcl.Range) *hcl.Diagnostic {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = "local." + name
	}

	return errorAt("Circular reference between locals", fmt.Sprintf("These locals refer to each other, so none of them can have a value: %s.", strings.Join(quoted, ", ")), subject)
}

// DuplicateLocal flags two locals entries sharing a name.
func DuplicateLocal(name string, previous, subject hcl.Range) *hcl.Diagnostic {
	return errorAt("Duplicate local value", fmt.Sprintf("A local named %q was already declared at %s.", name, previous.String()), subject)
}

// VarFileUnreadable flags a --var-file that could not be read from disk.
func VarFileUnreadable(path string, err error) *hcl.Diagnostic {
	return errorAt("Unable to read var file", fmt.Sprintf("The var file at %q could not be read: %s.", path, err), hcl.Range{})
}

// UndeclaredVarFileVariable flags a --var-file that sets a variable no block declares.
func UndeclaredVarFileVariable(name, path string, subject hcl.Range) *hcl.Diagnostic {
	return errorAt("Undefined variable", fmt.Sprintf("The var file %q sets %q, but the profile declares no such variable block.", path, name), subject)
}
