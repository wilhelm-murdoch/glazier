package diagnostics

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

// This file collects the diagnostics raised while resolving `variable` blocks
// and the --var flags that feed them. They are grouped here, separate from the
// schema validators in custom.go, because they concern the variable contract
// (declaration, typing, required-ness) rather than a single attribute's value.

// UndefinedVariableDiagnostic reports a --var flag that names a variable the
// profile never declares. The flag has no position in the source, so the
// diagnostic carries no subject range; the message names the offending key and
// points the author at the fix.
func UndefinedVariableDiagnostic(name string) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Undefined variable",
		Detail: fmt.Sprintf(
			`A value for %q was passed with --var, but the profile declares no variable %q. Add a variable %q {} block, or remove the flag.`,
			name, name, name,
		),
	}
}

// RequiredVariableDiagnostic reports a declared variable that has no default
// and was not supplied via --var. A variable is required precisely when it
// omits a default, mirroring Terraform.
func RequiredVariableDiagnostic(name string, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Missing required variable",
		Detail: fmt.Sprintf(
			`The variable %q has no default, so a value must be supplied with --var %s=<value>.`,
			name, name,
		),
		Subject: subject.Ptr(),
	}
}

// InvalidVariableTypeDiagnostic reports a variable whose `type` is not one of
// the supported primitive keywords. keyword is the offending text, or empty
// when the type was not a bare keyword at all (e.g. a quoted string).
func InvalidVariableTypeDiagnostic(name, keyword string, subject hcl.Range) *hcl.Diagnostic {
	detail := fmt.Sprintf(
		`The variable %q must declare its type as one of the bare keywords string, number or bool.`,
		name,
	)
	if keyword != "" {
		detail = fmt.Sprintf(
			`The variable %q declares an unsupported type %q; use one of the bare keywords string, number or bool.`,
			name, keyword,
		)
	}

	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable type",
		Detail:   detail,
		Subject:  subject.Ptr(),
	}
}

// InvalidVariableDescriptionDiagnostic reports a description that is not a
// literal string. Descriptions are static labels and cannot reference other
// variables or call functions.
func InvalidVariableDescriptionDiagnostic(name string, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable description",
		Detail:   fmt.Sprintf(`The description for variable %q must be a literal string.`, name),
		Subject:  subject.Ptr(),
	}
}

// InvalidVariableDefaultDiagnostic reports a default value that cannot be
// converted to the variable's declared type.
func InvalidVariableDefaultDiagnostic(name, typeName string, err error, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable default",
		Detail: fmt.Sprintf(
			`The default for variable %q does not match its declared type %s: %s.`,
			name, typeName, err,
		),
		Subject: subject.Ptr(),
	}
}

// InvalidVariableValueDiagnostic reports a --var value that cannot be coerced
// to the variable's declared type (e.g. --var count=abc for a number). The
// subject is the variable's declaration, since the flag itself has no source
// position.
func InvalidVariableValueDiagnostic(name, typeName string, err error, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid variable value",
		Detail: fmt.Sprintf(
			`The value passed for variable %q with --var cannot be used as %s: %s.`,
			name, typeName, err,
		),
		Subject: subject.Ptr(),
	}
}

// DuplicateVariableDiagnostic reports a variable name declared by more than one
// block, pointing at the later declaration and naming where it first appeared.
func DuplicateVariableDiagnostic(name string, previous, subject hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Duplicate variable",
		Detail: fmt.Sprintf(
			`The variable %q is declared more than once; it was first declared at %s.`,
			name, previous,
		),
		Subject: subject.Ptr(),
	}
}
