package harness

import (
	"fmt"
	"strings"
)

// Quote returns s as an HCL string literal. HCL reads the literal back as
// exactly s: no interpolation, no template directive.
func Quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case (r == '$' || r == '%') && strings.HasPrefix(s[i+1:], "{"):
			b.WriteRune(r)
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}

	b.WriteByte('"')
	return b.String()
}

// List returns the strings as an HCL list of string literals.
func List(items ...string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = Quote(s)
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}
