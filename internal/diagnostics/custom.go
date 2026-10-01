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
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
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

// LayoutDiagnostic validates a window layout. A layout is valid when it is
// either one of the named presets in list OR a structurally valid tmux layout
// coordinate string (e.g. "bb62,80x24,0,0"), which `save` captures verbatim
// from a live window as a fallback when no named preset applies. Anything else
// fails hard so a malformed value is caught before it reaches tmux.
func LayoutDiagnostic(field string, value cty.Value, list []string) hcl.Diagnostics {
	var out hcl.Diagnostics

	if value.IsNull() {
		return out
	}

	s := value.AsString()
	if slices.Contains(list, s) || enums.IsLayoutString(s) {
		return out
	}

	return hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  fmt.Sprintf(`Invalid %s specified`, field),
		Detail: fmt.Sprintf(
			`The %s value of "%s" is not a supported preset (%s) nor a valid tmux layout string.`,
			field,
			s,
			strings.Join(list, ", "),
		),
	}}
}

// SessionNameDiagnostic warns when tmux would rewrite characters in a session name.
func SessionNameDiagnostic(value cty.Value) hcl.Diagnostics {
	return renamedDiagnostic("session", `".", ":", "\", "$" or a control character`, value, tmux.SanitizeSessionName)
}

// NameDiagnostic warns when tmux would rewrite characters in a window or pane name.
func NameDiagnostic(kind string, value cty.Value) hcl.Diagnostics {
	return renamedDiagnostic(kind, `"\" or a control character`, value, tmux.SanitizeName)
}

// renamedDiagnostic warns when sanitize changes the name, because glaze then uses a different name.
func renamedDiagnostic(kind, chars string, value cty.Value, sanitize func(string) string) hcl.Diagnostics {
	if value.IsNull() || !value.IsKnown() {
		return nil
	}

	name := value.AsString()
	sanitized := sanitize(name)
	if sanitized == name {
		return nil
	}

	return hcl.Diagnostics{{
		Severity: hcl.DiagWarning,
		Summary:  fmt.Sprintf("%s name will be changed", strings.ToUpper(kind[:1])+kind[1:]),
		Detail: fmt.Sprintf(
			`tmux does not accept %s in a %s name, so glaze replaces them with "-". The %s %q will have the name %q.`,
			chars,
			kind,
			kind,
			name,
			sanitized,
		),
	}}
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
