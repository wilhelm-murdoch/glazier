package diagnostics

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

// This file collects the diagnostics raised while resolving `variable` and
// `locals` blocks, the --var flags, and the --var-file that feed them. They
// are grouped here, apart from the schema validators in custom.go, because
// they concern the variable contract (declaration, typing, required-ness)
// rather than a single attribute's value.

// DuplicateVariable flags two variable blocks sharing a name.
func DuplicateVariable(name string, previous, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Duplicate variable declaration",
		Detail:   fmt.Sprintf("A variable named %q was already declared at %s.", name, previous.String()),
		Subject:  &subject,
	}
}

// InvalidVariableType flags a variable block with an unsupported type. The
// keyword is the offending text, or empty when `type` was not a bare keyword
// at all (e.g. a quoted string).
func InvalidVariableType(name, keyword string, subject hcl.Range) *hcl.Diagnostic {
	detail := fmt.Sprintf("Variable %q must declare its type as one of the bare keywords string, number or bool.", name)
	if keyword != "" {
		detail = fmt.Sprintf("Variable %q declares unsupported type %q. Supported types are: string, number, bool.", name, keyword)
	}

	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable type",
		Detail:   detail,
		Subject:  &subject,
	}
}

// InvalidVariableDescription flags a variable description that is not a
// literal string.
func InvalidVariableDescription(name string, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable description",
		Detail:   fmt.Sprintf("The description of variable %q must be a literal string.", name),
		Subject:  &subject,
	}
}

// InvalidVariableDefault flags a default that cannot convert to the declared
// type.
func InvalidVariableDefault(name, keyword string, err error, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable default",
		Detail:   fmt.Sprintf("The default for variable %q is not a valid %s: %s.", name, keyword, err),
		Subject:  &subject,
	}
}

// UndefinedVariable flags a --var flag naming a variable no block declares.
func UndefinedVariable(name string) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Undefined variable",
		Detail:   fmt.Sprintf("A value was supplied for variable %q, but the profile declares no such variable block.", name),
	}
}

// InvalidVariableValue flags a supplied value that cannot convert to the
// declared type.
func InvalidVariableValue(name, friendlyType string, err error, subject hcl.Range) *hcl.Diagnostic {
	diag := &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable value",
		Detail:   fmt.Sprintf("The value supplied for variable %q is not a valid %s: %s.", name, friendlyType, err),
	}
	if subject.Filename != "" {
		diag.Subject = &subject
	}
	return diag
}

// RequiredVariable flags a declared variable with neither a supplied value
// nor a default.
func RequiredVariable(name string, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Required variable not set",
		Detail:   fmt.Sprintf("Variable %q declares no default, so a value must be supplied with --var %s=... or via --var-file.", name, name),
		Subject:  &subject,
	}
}

// DuplicateLocal flags two locals entries sharing a name.
func DuplicateLocal(name string, previous, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Duplicate local value",
		Detail:   fmt.Sprintf("A local named %q was already declared at %s.", name, previous.String()),
		Subject:  &subject,
	}
}

// VarFileUnreadable flags a --var-file that could not be read from disk.
func VarFileUnreadable(path string, err error) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Unable to read var file",
		Detail:   fmt.Sprintf("The var file at %q could not be read: %s.", path, err),
	}
}

// VarFileInvalid flags a --var-file whose contents failed to parse.
func VarFileInvalid(path, detail string) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid var file",
		Detail:   fmt.Sprintf("The var file at %q could not be parsed: %s.", path, detail),
	}
}

// UndeclaredVarFileVariable flags a --var-file that sets a variable no block
// declares.
func UndeclaredVarFileVariable(name, path string, subject hcl.Range) *hcl.Diagnostic {
	diag := &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Undefined variable",
		Detail:   fmt.Sprintf("The var file %q sets %q, but the profile declares no such variable block.", path, name),
	}
	if subject.Filename != "" {
		diag.Subject = &subject
	}
	return diag
}
