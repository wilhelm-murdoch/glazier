package diagnostics

import (
	"fmt"
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

// Invalid returns the error for a field value that glaze does not accept.
func Invalid(field, detail string, args ...any) hcl.Diagnostics {
	return hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  fmt.Sprintf("Invalid %s specified", field),
		Detail:   fmt.Sprintf(detail, args...),
	}}
}

// ContainsDiagnostic rejects a value that is not in list.
func ContainsDiagnostic(field string, value cty.Value, list []string) hcl.Diagnostics {
	if value.IsNull() || slices.Contains(list, value.AsString()) {
		return nil
	}

	return Invalid(field, `The %s value of "%s" is not supported among: %s.`, field, value.AsString(), strings.Join(list, ", "))
}

// LayoutDiagnostic accepts a named preset from list or a raw tmux layout string, which `save` writes when no preset matches.
func LayoutDiagnostic(field string, value cty.Value, list []string) hcl.Diagnostics {
	if value.IsNull() || slices.Contains(list, value.AsString()) || enums.IsLayoutString(value.AsString()) {
		return nil
	}

	return Invalid(field, `The %s value of "%s" is not a supported preset (%s) nor a valid tmux layout string.`, field, value.AsString(), strings.Join(list, ", "))
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

// DirectoryDiagnostic rejects a path that does not exist or is not a directory.
func DirectoryDiagnostic(field string, value cty.Value) hcl.Diagnostics {
	if value.IsNull() {
		return nil
	}

	if info, err := os.Stat(files.ExpandPath(value.AsString())); err == nil && info.IsDir() {
		return nil
	}

	return Invalid(field, `The %s of "%s" does not exist or is not a directory.`, field, value.AsString())
}

// sizePattern matches a size in cells, for example "20", or as a percentage, for example "25%".
var sizePattern = regexp.MustCompile(`^(\d+)\s*%$|^(\d+)$`)

// WrongSizeDiagnostic rejects a size that is not a number of cells or a percentage. The spec makes every size a string.
func WrongSizeDiagnostic(field string, value cty.Value) hcl.Diagnostics {
	if value.IsNull() || sizePattern.MatchString(value.AsString()) {
		return nil
	}

	return Invalid(field, `The %s value "%s" should be a number of cells or a percentage, for example 20 or 25%%.`, field, value.AsString())
}
