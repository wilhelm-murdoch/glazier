package diagnostics

import (
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/pkg/files"
)

// ContainsDiagnostic is responsible for checking if a value is present in a given list and returning a diagnostic if not.
func ContainsDiagnostic(field string, value cty.Value, list []string) hcl.Diagnostics {
	var out hcl.Diagnostics

	if !value.IsNull() && !slices.Contains(list, value.AsString()) {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`Invalid %s specified`, field),
			Detail: fmt.Sprintf(
				`The %s value of "%s" is not supported among: %s.`,
				field,
				value.AsString(),
				strings.Join(list, ", "),
			),
		}}
	}

	return out
}

// DirectoryDiagnostic is responsible for checking if a given value is a valid directory and returning a diagnostic if not.
func DirectoryDiagnostic(field string, value cty.Value) hcl.Diagnostics {
	var out hcl.Diagnostics

	if !value.IsNull() {
		fileInfo, err := os.Stat(files.ExpandPath(value.AsString()))
		if err != nil || errors.Is(err, fs.ErrNotExist) || !fileInfo.IsDir() {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf(`Invalid %s specified`, field),
				Detail: fmt.Sprintf(
					`The %s of "%s" does not exist or is not a directory.`,
					field,
					value.AsString(),
				),
			}}
		}
	}

	return out
}

// FileDiagnostic is responsible for checking if a given value is a valid file and returning a diagnostic if not.
func FileDiagnostic(field string, value cty.Value) hcl.Diagnostics {
	var out hcl.Diagnostics

	if !value.IsNull() {
		fileInfo, err := os.Stat(files.ExpandPath(value.AsString()))
		if err != nil || errors.Is(err, fs.ErrNotExist) || fileInfo.IsDir() {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf(`Invalid %s specified`, field),
				Detail: fmt.Sprintf(
					`The %s of "%s" does not exist, cannot be accessed or is a directory.`,
					field,
					value.AsString(),
				),
			}}
		}
	}

	return out
}

// WrongAttributeDiagnostic is responsible for returning a diagnostic for an incorrect attribute value.
func WrongAttributeDiagnostic(field, have, want string) hcl.Diagnostic {
	return hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  fmt.Sprintf(`Invalid %s specified`, field),
		Detail:   fmt.Sprintf(`The %s value "%s" should be "%s".`, field, have, want),
	}
}

// WrongSizeDiagnostic is used to determine whether a size value resolves to either a positive integer or a valid percentage string.
func WrongSizeDiagnostic(field string, value cty.Value) hcl.Diagnostics {
	var out hcl.Diagnostics

	if value.IsNull() {
		return nil
	}

	switch value.Type() {
	case cty.Number:
		f := value.AsBigFloat()
		i, acc := f.Int64()

		if acc != big.Exact || i <= 0 {
			out = out.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf(`Invalid %s specified`, field),
				Detail: fmt.Sprintf(
					`The %s value "%s" should be a positive integer.`,
					field,
					f.String(),
				),
			})
		}
	case cty.String:
		matched, _ := regexp.MatchString(
			`^(\d+)\s*%$|^(\d+)$`,
			value.AsString(),
		)

		if !matched {
			out = out.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf(`Invalid %s specified`, field),
				Detail: fmt.Sprintf(
					`The %s value "%s" should be a valid percentage.`,
					field,
					value.AsString(),
				),
			})
		}
	default:
		out = out.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`Invalid %s specified`, field),
			Detail: fmt.Sprintf(
				`The %s value must be an integer or percentage.`,
				field,
			),
		})
	}

	return out
}
