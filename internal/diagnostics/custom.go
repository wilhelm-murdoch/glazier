package diagnostics

import (
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// sizePattern matches a whole number of cells, for example "20", or a percentage, for example "25%".
var sizePattern = regexp.MustCompile(`^(\d+)(%?)$`)

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

// SessionNameDiagnostic rejects an empty session name, which some tmux versions refuse, and warns when tmux would rewrite characters in it.
func SessionNameDiagnostic(value cty.Value) hcl.Diagnostics {
	if value.IsKnown() && !value.IsNull() && value.AsString() == "" {
		return Invalid("session name", `The session name must not be empty. Leave out "name" to use "default".`)
	}

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

// DirectoryDiagnostic rejects a path that glaze cannot expand, or that does not exist or is not a directory.
// A relative path is checked against baseDirectory, the directory of the profile.
func DirectoryDiagnostic(field string, value cty.Value, baseDirectory string) hcl.Diagnostics {
	if value.IsNull() {
		return nil
	}

	path, err := files.ResolveDirectory(value.AsString(), baseDirectory)
	if err != nil {
		return Invalid(field, `The %s of "%s" is not valid: %s.`, field, value.AsString(), err)
	}

	if info, err := os.Stat(path); value.AsString() != "" && err == nil && info.IsDir() {
		return nil
	}

	return Invalid(field, `The %s of "%s" does not exist or is not a directory.`, field, value.AsString())
}

// parseSize returns the number in a size and whether it is a percentage. ok is false when the value is not a size.
func parseSize(value string) (n int, percent, ok bool) {
	match := sizePattern.FindStringSubmatch(value)
	if match == nil {
		return 0, false, false
	}

	n, err := strconv.Atoi(match[1])

	return n, match[2] == "%", err == nil
}

// SizeDiagnostic rejects a size that is not 1 or more cells, or 1% to 100%. The spec makes every size a string.
func SizeDiagnostic(field string, value cty.Value) hcl.Diagnostics {
	if value.IsNull() {
		return nil
	}

	if n, percent, ok := parseSize(value.AsString()); ok && n >= 1 && (!percent || n <= 100) {
		return nil
	}

	return Invalid(field, `The %s value "%s" should be 1 or more cells, for example 20, or a percentage from 1%% to 100%%, for example 25%%.`, field, value.AsString())
}

// AmountDiagnostic rejects an adjust amount that is not 1 or more cells, because tmux resizes a pane in a direction by cells only.
func AmountDiagnostic(field string, value cty.Value) hcl.Diagnostics {
	if value.IsNull() {
		return nil
	}

	if n, percent, ok := parseSize(value.AsString()); ok && n >= 1 && !percent {
		return nil
	}

	return Invalid(field, `The %s value "%s" should be 1 or more cells, for example 5. A percentage is not supported here.`, field, value.AsString())
}

// RequiredDiagnostic rejects a null value for a field that must have a value.
func RequiredDiagnostic(field string, value cty.Value) hcl.Diagnostics {
	if !value.IsNull() {
		return nil
	}

	return Invalid(field, `The %s must have a value, not null.`, field)
}

// NoNullsDiagnostic rejects a list or a map that holds a null, because glaze cannot pass a null to tmux.
func NoNullsDiagnostic(field string, value cty.Value) hcl.Diagnostics {
	if value.IsNull() || !value.IsKnown() || !value.CanIterateElements() {
		return nil
	}

	for it := value.ElementIterator(); it.Next(); {
		key, element := it.Element()
		if !element.IsNull() {
			continue
		}

		// A list has number keys and a map has string keys.
		var where string
		if key.Type() == cty.Number {
			where = fmt.Sprintf("index %s", key.AsBigFloat().String())
		} else {
			where = fmt.Sprintf("key %q", key.AsString())
		}

		return Invalid(field, `The %s must not contain null (%s). Remove the element or give it a value.`, field, where)
	}

	return nil
}

// HooksDiagnostic rejects a null hook command and a hook name that no tmux from 3.2a to 3.7c knows.
func HooksDiagnostic(value cty.Value) hcl.Diagnostics {
	if diags := NoNullsDiagnostic("hooks", value); diags.HasErrors() {
		return diags
	}

	if value.IsNull() || !value.IsKnown() {
		return nil
	}

	var diags hcl.Diagnostics
	for _, name := range slices.Sorted(maps.Keys(value.AsValueMap())) {
		if !enums.IsHook(name) {
			diags = diags.Extend(Invalid("hook", `No tmux from 3.2a to 3.7c knows the hook "%s". Check the name, for example "session-created" or "after-new-window".`, name))
		}
	}

	return diags
}

// LayoutCellsDiagnostic rejects a raw layout string that describes a different number of panes than the window declares.
// tmux applies such a layout only in part, or rejects it while `up` builds the session.
func LayoutCellsDiagnostic(window cty.Value) hcl.Diagnostics {
	if window.IsNull() || !window.IsKnown() {
		return nil
	}

	layout, panes := window.GetAttr("layout"), window.GetAttr("panes")
	if layout.IsNull() || !layout.IsKnown() || !enums.IsLayoutString(layout.AsString()) || panes.IsNull() || !panes.IsKnown() {
		return nil
	}

	cells, declared := enums.LayoutCellCount(layout.AsString()), panes.LengthInt()
	if cells == declared {
		return nil
	}

	return Invalid("layout", `The raw layout describes %d panes, but the window declares %d. tmux applies such a layout only in part, or rejects it.`, cells, declared)
}
