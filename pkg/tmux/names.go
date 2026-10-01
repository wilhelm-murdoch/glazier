package tmux

import (
	"strings"
	"unicode"
)

// sessionNameReplacer replaces the characters that tmux rewrites only in session names.
// `$` is replaced on every version so that one profile gives the same name everywhere.
var sessionNameReplacer = strings.NewReplacer(".", "-", ":", "-", "$", "-")

// SanitizeName replaces each backslash and control character with a hyphen, because tmux 3.7 and later store them escaped.
func SanitizeName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '\\' || unicode.IsControl(r) {
			return '-'
		}

		return r
	}, name)
}

// SanitizeSessionName returns a session name that tmux stores unchanged.
func SanitizeSessionName(sessionName string) string {
	return SanitizeName(sessionNameReplacer.Replace(sessionName))
}

// escapeFormat doubles each # so tmux does not expand a name, title or directory as a format.
// tmux keeps a run of # before [ as it is, because #[ starts a style, so that run is not doubled.
func escapeFormat(s string) string {
	var out strings.Builder

	for i := 0; i < len(s); {
		if s[i] != '#' {
			out.WriteByte(s[i])
			i++
			continue
		}

		end := i
		for end < len(s) && s[end] == '#' {
			end++
		}

		run := s[i:end]
		if end < len(s) && s[end] == '[' {
			out.WriteString(run)
		} else {
			out.WriteString(run + run)
		}

		i = end
	}

	return out.String()
}

// posixQuote returns s as one single-quoted word for a POSIX shell.
func posixQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
