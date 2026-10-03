package cases

import "strings"

// commas joins the lines of tmux output with commas, like paste -sd, in a shell.
func commas(s string) string { return strings.ReplaceAll(s, "\n", ",") }

// firstLine returns the first line of s.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
