package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The placeholders that Normalize puts in place of the paths of a run.
const (
	WorkPlaceholder = "$WORK"
	HomePlaceholder = "$HOME"
)

// logTimestamp is the time at the start of a log line of glaze.
var logTimestamp = regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} `)

// Golden checks got against golden/<name> after Normalize. With -update, it
// writes got to the file instead.
func (c *Case) Golden(label, name, got string) bool {
	c.t.Helper()
	path := filepath.Join(c.env.Root, "golden", filepath.FromSlash(name))
	got = c.Normalize(got)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
			c.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), fileMode); err != nil { // #nosec G306 -- a golden file in the repository
			c.t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) // #nosec G304 -- a golden file of the module
	if err != nil {
		return c.record(label, false, fmt.Sprintf("read %s: %v; run the case with -update to write it", name, err))
	}
	return c.record(label, got == string(want), fmt.Sprintf("differs from golden/%s:\n%s", name, diff(string(want), got)))
}

// Normalize replaces the paths of the run with placeholders and removes the
// time from each log line, so that output compares equal on every run.
func (c *Case) Normalize(s string) string {
	s = strings.ReplaceAll(s, c.Home, HomePlaceholder)
	s = strings.ReplaceAll(s, c.Dir, WorkPlaceholder)
	return logTimestamp.ReplaceAllString(s, "")
}

// diff shows the first line that differs, with its neighbours.
func diff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("line %d:\n- %q\n+ %q", i+1, wl, gl)
		}
	}
	return "(no difference in lines; check the final newline)"
}
