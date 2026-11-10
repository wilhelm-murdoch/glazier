package actions

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rendered returns the report of a plan as one string.
func rendered(t *testing.T, p plan, inside bool) string {
	t.Helper()

	var b bytes.Buffer
	require.NoError(t, render(&b, p, inside, false))

	return b.String()
}

func TestRenderNoChanges(t *testing.T) {
	p := plan{root: item{kind: "session", name: "api"}}

	// The sentence, then the two fixed lines, and no tree, Plan line or hint.
	assert.Equal(t, fmt.Sprintf(noChangesFormat, `"api"`)+"\n"+notComparedProvenance+"\n"+notComparedGeometry+"\n", rendered(t, p, false))
	assert.Equal(t, rendered(t, p, false), rendered(t, p, true), "inside or outside the session makes no difference without a hint")
}

func TestRenderPlanLine(t *testing.T) {
	root := item{mark: markAdd, kind: "session", name: "api"}

	t.Run("leaves out to check when nothing is to check", func(t *testing.T) {
		lines := report(t, plan{root: root, adds: 1, changes: 2, removes: 3}, false)

		assert.Equal(t, "Plan: 1 to add, 2 to change, 3 to remove.", lines[1])
	})

	t.Run("appends to check when something is", func(t *testing.T) {
		lines := report(t, plan{root: root, adds: 1, changes: 2, removes: 3, toCheck: 4}, false)

		assert.Equal(t, "Plan: 1 to add, 2 to change, 3 to remove, 4 to check.", lines[1])
	})

	t.Run("counts zero for a plan with only questions", func(t *testing.T) {
		questioned := item{kind: "session", name: "api", lines: []line{{markCheck, "focus: x"}}}
		lines := report(t, plan{root: questioned, toCheck: 1}, false)

		assert.Equal(t, "Plan: 0 to add, 0 to change, 0 to remove, 1 to check.", lines[2])
	})
}

func TestRenderHints(t *testing.T) {
	changed := item{kind: "session", name: "api", lines: []line{{markChange, "order: b, a -> a, b"}}}
	questioned := item{kind: "session", name: "api", lines: []line{{markCheck, "focus: x"}}}
	created := item{mark: markAdd, kind: "session", name: "api", column: "/srv"}

	cases := []struct {
		name   string
		p      plan
		inside bool
		hint   string
	}{
		{"a difference outside the session gives the apply hint", plan{root: changed, changes: 1}, false, hintApply},
		{"a difference inside the session gives the inside hint", plan{root: changed, changes: 1}, true, hintApplyInside},
		{"an addition gives the apply hint", plan{root: item{kind: "session", name: "api", children: []item{{mark: markAdd, kind: "window", name: "w", pos: 1}}}, adds: 1}, false, hintApply},
		{"a creation plan gives the create hint", plan{root: created, creation: true, adds: 1}, false, hintCreate},
		{"a creation plan inside tmux still gives the create hint", plan{root: created, creation: true, adds: 1}, true, hintCreate},
		{"questions alone give no hint", plan{root: questioned, toCheck: 1}, false, ""},
		{"questions alone inside the session give no hint", plan{root: questioned, toCheck: 1}, true, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines := report(t, c.p, c.inside)

			// The two fixed lines come right before the hint, or end the report when there is none.
			if c.hint == "" {
				assert.Equal(t, []string{notComparedProvenance, notComparedGeometry}, lines[len(lines)-2:])
			} else {
				assert.Equal(t, []string{notComparedProvenance, notComparedGeometry, c.hint}, lines[len(lines)-3:])
			}
		})
	}
}

func TestRenderTree(t *testing.T) {
	p := plan{
		root: item{
			kind:  "session",
			name:  "api",
			lines: []line{{markChange, "order: run, edit -> edit, run"}, {markCheck, "focus: window \"run\" [2], profile window \"edit\" [1]; " + reasonFocusWindow}},
			children: []item{
				{kind: "window", name: "edit", pos: 1, children: []item{
					{kind: "pane", name: "shell", pos: 1},
					{kind: "pane", name: "tests", pos: 2, lines: []line{{markCheck, "title: \"psql\", profile name \"tests\"; " + reasonTitle}}},
					{mark: markAdd, kind: "pane", name: "logs", pos: 3, column: "/var/log"},
					{mark: markRemove, kind: "pane", name: "", pos: 4},
				}},
				{kind: "window", name: "run", pos: 2, children: []item{{kind: "pane", name: "server", pos: 1}}},
				{mark: markRemove, kind: "window", name: "zsh", pos: 3, note: labelNamedByTmux, children: []item{{mark: markRemove, kind: "pane", name: "tail", pos: 1}}},
			},
		},
		adds:    1,
		changes: 1,
		removes: 3,
		toCheck: 1,
	}

	// Two spaces per level, an attribute line one level under its item, and a matched item with nothing under it is left out.
	assert.Equal(t, []string{
		`session "api"`,
		`  ~ order: run, edit -> edit, run`,
		`  ? focus: window "run" [2], profile window "edit" [1]; ` + reasonFocusWindow,
		`  window "edit" [1]`,
		`    pane "tests" [2]`,
		`      ? title: "psql", profile name "tests"; ` + reasonTitle,
		`    + pane "logs" [3]  /var/log`,
		`    - pane ` + labelUnnamed + ` [4]`,
		`  - window "zsh" [3] ` + labelNamedByTmux,
		`    - pane "tail" [1]`,
		"Plan: 1 to add, 1 to change, 3 to remove, 1 to check.",
		notComparedProvenance,
		notComparedGeometry,
		hintApply,
	}, report(t, p, false))
}

func TestRenderHostileNames(t *testing.T) {
	p := plan{
		root: item{mark: markAdd, kind: "session", name: "a\"b;#c\x1b[31m\n", column: "/srv", children: []item{
			{mark: markAdd, kind: "window", name: "w\ttab", pos: 1, children: []item{
				{mark: markAdd, kind: "pane", name: "p\\q", pos: 1, column: "/srv/x y"},
			}},
		}},
		adds: 3,
	}

	out := rendered(t, p, false)

	// Each item takes one line, the escape byte is quoted away, and a newline in a name cannot forge a line.
	assert.Equal(t, []string{
		`+ session "a\"b;#c\x1b[31m\n"  /srv`,
		`  + window "w\ttab" [1]`,
		`    + pane "p\\q" [1]  /srv/x y`,
		"Plan: 3 to add, 0 to change, 0 to remove.",
		notComparedProvenance,
		notComparedGeometry,
		hintApply,
	}, strings.Split(strings.TrimSuffix(out, "\n"), "\n"))
	assert.NotContains(t, out, "\x1b")
	assert.NotContains(t, out, "\t")
}

func TestRenderHintsCarryNoSelection(t *testing.T) {
	// The hints repeat no name, path, variable or socket, so a golden holds no server-specific text and no secret reaches stdout.
	for _, hint := range []string{hintApply, hintApplyInside, hintCreate, notComparedProvenance, notComparedGeometry} {
		assert.NotContains(t, hint, "--var")
		assert.NotContains(t, hint, "--profile-path")
		assert.NotContains(t, hint, "--socket")
		assert.NotContains(t, hint, "/")
		assert.NotContains(t, hint, ".glaze")
		assert.NotContains(t, hint, "\n")
	}

	assert.Contains(t, hintApply, "glaze up --clear")
	assert.Contains(t, hintApplyInside, "glaze up --clear")
	assert.Contains(t, hintApplyInside, "including this shell")
	assert.Contains(t, hintCreate, "glaze up ")
	assert.NotContains(t, hintCreate, "--clear")
}
