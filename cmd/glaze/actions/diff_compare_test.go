package actions

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noChanges is the first line of a report without a difference, for the session api.
var noChanges = fmt.Sprintf(noChangesFormat, `"api"`)

// report renders a plan and returns its lines, without the empty string after the last newline.
func report(t *testing.T, p plan, inside bool) []string {
	t.Helper()

	var b bytes.Buffer
	require.NoError(t, render(&b, p, inside))

	return strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
}

// tree returns the lines of the report before its Plan line, or every line when there is none.
func tree(t *testing.T, p plan) []string {
	t.Helper()

	lines := report(t, p, false)
	for i, line := range lines {
		if strings.HasPrefix(line, "Plan: ") {
			return lines[:i]
		}
	}

	return lines
}

// The builders below keep the tables short: a profile session in /srv with a declared directory, and a live session in /srv.

func wantSession(windows ...desiredWindow) desired {
	return desired{Name: "api", Directory: "/srv", DirectoryDeclared: true, Focus: -1, Windows: windows}
}

func wantWindow(name string, focus int, panes ...desiredPane) desiredWindow {
	return desiredWindow{Name: name, Focus: focus, Panes: panes}
}

func autoDeclared(name string, panes ...desiredPane) desiredWindow {
	return desiredWindow{Name: name, AutomaticDeclared: true, Focus: -1, Panes: panes}
}

func wantPane(name, dir string) desiredPane {
	return desiredPane{Name: name, Directory: dir}
}

func liveSession(windows ...savedWindow) savedSession {
	return savedSession{Name: "api", StartingDirectory: "/srv", Windows: windows}
}

func liveWindow(name string, active bool, panes ...savedPane) savedWindow {
	return savedWindow{Name: name, LiveName: name, Layout: "bb62,80x24,0,0", Focus: active, Panes: panes}
}

func autoWindow(liveName string, active bool, panes ...savedPane) savedWindow {
	return savedWindow{LiveName: liveName, Automatic: true, Layout: "bb62,80x24,0,0", Focus: active, Panes: panes}
}

func livePane(title, dir string, active bool) savedPane {
	return savedPane{Name: title, Title: title, StartingDirectory: dir, Focus: active}
}

// hostPane is a pane that tmux titled with the host name, which save drops and diff reads as a pane glaze did not create.
func hostPane(dir string, active bool) savedPane {
	return savedPane{Title: "buildhost", HostTitle: true, StartingDirectory: dir, Focus: active}
}

func TestCompareWorkedExample(t *testing.T) {
	// The profile: session api in /srv/api; edit (focus) with shell and tests (focus); run with server and logs in /var/log; solo with f; misc with m.
	want := desired{
		Name:              "api",
		Directory:         "/srv/api",
		DirectoryDeclared: true,
		Focus:             0,
		Windows: []desiredWindow{
			wantWindow("edit", 1, wantPane("shell", "/srv/api"), wantPane("tests", "/srv/api")),
			wantWindow("run", -1, wantPane("server", "/srv/api"), wantPane("logs", "/var/log")),
			wantWindow("solo", -1, wantPane("f", "/srv/api")),
			wantWindow("misc", -1, wantPane("m", "/srv/api")),
		},
	}

	// The session: run selected, cd in shell, tests retitled psql, a third pane split in edit and active, run's panes replaced by a bare split, solo killed, misc renamed, extra opened.
	live := savedSession{
		Name:              "api",
		StartingDirectory: "/srv/api",
		Windows: []savedWindow{
			liveWindow("edit", false, livePane("shell", "/srv/api/cmd", false), livePane("psql", "/srv/api", false), hostPane("/srv/api", true)),
			liveWindow("run", true, hostPane("/srv/api", true)),
			liveWindow("notes", false, livePane("m", "/srv/api", true)),
			liveWindow("extra", false, hostPane("/srv/api", true)),
		},
	}

	p := compare(want, live)

	assert.Equal(t, []string{
		`session "api"`,
		`  ? focus: window "run" [2], profile window "edit" [1]; ` + reasonFocusWindow,
		`  window "edit" [1]`,
		`    ? focus: pane ` + labelUnnamed + ` [3], profile pane "tests" [2]; ` + reasonFocusPane,
		`    pane "shell" [1]`,
		`      ? directory: "/srv/api/cmd", profile "/srv/api"; ` + reasonDirectory,
		`    pane "tests" [2]`,
		`      ? title: "psql", profile name "tests"; ` + reasonTitle,
		`    - pane ` + labelUnnamed + ` [3]`,
		`  window "run" [2]`,
		`    pane "server" [1]`,
		`      ~ title: ` + labelUnnamed + ` -> "server"; ` + reasonReplaced,
		`    + pane "logs" [2]  /var/log`,
		`  + window "solo" [3]`,
		`    + pane "f" [1]  /srv/api`,
		`  + window "misc" [4]`,
		`    + pane "m" [1]  /srv/api`,
		`  - window "notes" [3]`,
		`    - pane "m" [1]`,
		`  - window "extra" [4]`,
		`    - pane ` + labelUnnamed + ` [1]`,
		"Plan: 5 to add, 1 to change, 5 to remove, 4 to check.",
		notComparedProvenance,
		notComparedGeometry,
		hintApply,
	}, report(t, p, false))

	assert.Equal(t, 5, p.adds)
	assert.Equal(t, 1, p.changes)
	assert.Equal(t, 5, p.removes)
	assert.Equal(t, 4, p.toCheck)
	assert.False(t, p.creation)
}

func TestCompare(t *testing.T) {
	cases := []struct {
		name string
		want desired
		live savedSession
		tree []string

		adds, changes, removes, toCheck int
	}{
		{
			name: "identical sides give an empty plan",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv"))),
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", true))),
			tree: []string{noChanges, notComparedProvenance, notComparedGeometry},
		},
		{
			name: "a profile window without a live match is added with its panes",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv")), wantWindow("run", -1, wantPane("server", "/srv"), wantPane("logs", "/var/log"))),
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  + window "run" [2]`,
				`    + pane "server" [1]  /srv`,
				`    + pane "logs" [2]  /var/log`,
			},
			adds: 3,
		},
		{
			name: "a live window without a match is removed after the profile windows",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv")), wantWindow("run", -1, wantPane("server", "/srv"))),
			live: liveSession(liveWindow("extra", false, livePane("tail", "/srv", true)), liveWindow("edit", true, livePane("shell", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  + window "run" [2]`,
				`    + pane "server" [1]  /srv`,
				`  - window "extra" [1]`,
				`    - pane "tail" [1]`,
			},
			adds:    2,
			removes: 2,
		},
		{
			name: "a rename is one removal and one addition",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv"))),
			live: liveSession(liveWindow("renamed", true, livePane("shell", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  + window "edit" [1]`,
				`    + pane "shell" [1]  /srv`,
				`  - window "renamed" [1]`,
				`    - pane "shell" [1]`,
			},
			adds:    2,
			removes: 2,
		},
		{
			name: "duplicate window names pair in order",
			want: wantSession(wantWindow("w", -1, wantPane("a", "/srv")), wantWindow("w", -1, wantPane("b", "/srv"), wantPane("c", "/srv"))),
			live: liveSession(liveWindow("w", true, livePane("a", "/srv", true)), liveWindow("w", false, livePane("b", "/srv", true), livePane("c", "/srv", false))),
			tree: []string{noChanges, notComparedProvenance, notComparedGeometry},
		},
		{
			name: "a live window that tmux named never matches a named profile window",
			want: wantSession(wantWindow("zsh", -1, wantPane("shell", "/srv"))),
			live: liveSession(autoWindow("zsh", true, livePane("shell", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  + window "zsh" [1]`,
				`    + pane "shell" [1]  /srv`,
				`  - window "zsh" [1] ` + labelNamedByTmux,
				`    - pane "shell" [1]`,
			},
			adds:    2,
			removes: 2,
		},
		{
			name: "a profile window under automatic-rename matches the live window that tmux named and prints nothing",
			want: wantSession(autoDeclared("default", wantPane("shell", "/srv"))),
			live: liveSession(autoWindow("vim", true, livePane("shell", "/srv", true))),
			tree: []string{noChanges, notComparedProvenance, notComparedGeometry},
		},
		{
			name: "two profile windows under automatic-rename pair with the live ones in order",
			want: wantSession(autoDeclared("one", wantPane("a", "/srv")), autoDeclared("two", wantPane("b", "/srv"), wantPane("c", "/srv"))),
			live: liveSession(autoWindow("zsh", true, livePane("a", "/srv", true)), autoWindow("vim", false, livePane("b", "/srv", true), livePane("c", "/srv", false))),
			tree: []string{noChanges, notComparedProvenance, notComparedGeometry},
		},
		{
			name: "matched windows in another order are one change",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv")), wantWindow("run", -1, wantPane("server", "/srv"))),
			live: liveSession(liveWindow("run", true, livePane("server", "/srv", true)), liveWindow("edit", false, livePane("shell", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  ~ order: run, edit -> edit, run`,
			},
			changes: 1,
		},
		{
			name: "an unmatched live window does not move the order",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv")), wantWindow("run", -1, wantPane("server", "/srv"))),
			live: liveSession(liveWindow("extra", false, livePane("tail", "/srv", true)), liveWindow("edit", true, livePane("shell", "/srv", true)), liveWindow("run", false, livePane("server", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  - window "extra" [1]`,
				`    - pane "tail" [1]`,
			},
			removes: 2,
		},
		{
			name: "an extra live pane with the host title is removed as unnamed",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv"))),
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", false), hostPane("/srv", true))),
			tree: []string{
				`session "api"`,
				`  window "edit" [1]`,
				`    - pane ` + labelUnnamed + ` [2]`,
			},
			removes: 1,
		},
		{
			name: "an extra profile pane carries its directory",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv"), wantPane("logs", "/var/log"))),
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  window "edit" [1]`,
				`    + pane "logs" [2]  /var/log`,
			},
			adds: 1,
		},
		{
			name: "an extra profile pane that inherits an undeclared session directory carries the current directory token",
			want: desired{Name: "api", Directory: "/home/v", Focus: -1, Windows: []desiredWindow{wantWindow("edit", -1, desiredPane{Name: "shell", Directory: "/home/v", InheritsSessionDirectory: true}, desiredPane{Name: "logs", Directory: "/home/v", InheritsSessionDirectory: true})}},
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  ? starting_directory: "/srv", profile declares none; up would use the current directory "/home/v"`,
				`  window "edit" [1]`,
				`    + pane "logs" [2]  (current directory)`,
			},
			adds:    1,
			toCheck: 1,
		},
		{
			name: "a host title at a matched position is a change",
			want: wantSession(wantWindow("edit", -1, wantPane("server", "/srv"))),
			live: liveSession(liveWindow("edit", true, hostPane("/srv", true))),
			tree: []string{
				`session "api"`,
				`  window "edit" [1]`,
				`    pane "server" [1]`,
				`      ~ title: ` + labelUnnamed + ` -> "server"; ` + reasonReplaced,
			},
			changes: 1,
		},
		{
			name: "a host title against the default name is still a change",
			want: wantSession(wantWindow("edit", -1, wantPane("default", "/srv"))),
			live: liveSession(liveWindow("edit", true, hostPane("/srv", true))),
			tree: []string{
				`session "api"`,
				`  window "edit" [1]`,
				`    pane "default" [1]`,
				`      ~ title: ` + labelUnnamed + ` -> "default"; ` + reasonReplaced,
			},
			changes: 1,
		},
		{
			name: "another title is a question",
			want: wantSession(wantWindow("edit", -1, wantPane("tests", "/srv"))),
			live: liveSession(liveWindow("edit", true, livePane("psql", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  window "edit" [1]`,
				`    pane "tests" [1]`,
				`      ? title: "psql", profile name "tests"; ` + reasonTitle,
			},
			toCheck: 1,
		},
		{
			name: "a pane directory that differs is a question",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv"))),
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv/cmd", true))),
			tree: []string{
				`session "api"`,
				`  window "edit" [1]`,
				`    pane "shell" [1]`,
				`      ? directory: "/srv/cmd", profile "/srv"; ` + reasonDirectory,
			},
			toCheck: 1,
		},
		{
			name: "a pane directory that no longer exists says so",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv"))),
			live: liveSession(liveWindow("edit", true, savedPane{Name: "shell", Title: "shell", StartingDirectory: "/srv/gone", DirectoryMissing: true, Focus: true})),
			tree: []string{
				`session "api"`,
				`  window "edit" [1]`,
				`    pane "shell" [1]`,
				`      ? directory: "/srv/gone" ` + labelNoLongerHere + `, profile "/srv"; ` + reasonDirectory,
			},
			toCheck: 1,
		},
		{
			name: "a pane directory that tmux did not report gives no line",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv"))),
			live: liveSession(liveWindow("edit", true, savedPane{Name: "shell", Title: "shell", DirectoryUnknown: true, Focus: true})),
			tree: []string{noChanges, notComparedProvenance, notComparedGeometry},
		},
		{
			name: "a pane that inherits an undeclared session directory is compared with the live session path",
			want: desired{Name: "api", Directory: "/home/v", Focus: -1, Windows: []desiredWindow{wantWindow("edit", -1, desiredPane{Name: "shell", Directory: "/home/v", InheritsSessionDirectory: true}, desiredPane{Name: "logs", Directory: "/home/v", InheritsSessionDirectory: true})}},
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", true), livePane("logs", "/srv/cmd", false))),
			tree: []string{
				`session "api"`,
				`  ? starting_directory: "/srv", profile declares none; up would use the current directory "/home/v"`,
				`  window "edit" [1]`,
				`    pane "logs" [2]`,
				`      ? directory: "/srv/cmd", profile "/srv"; ` + reasonDirectory,
			},
			toCheck: 2,
		},
		{
			name: "an undeclared session directory equal to the live one gives no line",
			want: desired{Name: "api", Directory: "/srv", Focus: -1, Windows: []desiredWindow{wantWindow("edit", -1, desiredPane{Name: "shell", Directory: "/srv", InheritsSessionDirectory: true})}},
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", true))),
			tree: []string{noChanges, notComparedProvenance, notComparedGeometry},
		},
		{
			name: "a declared session directory that differs is a change",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv"))),
			live: savedSession{Name: "api", StartingDirectory: "/old", Windows: []savedWindow{liveWindow("edit", true, livePane("shell", "/srv", true))}},
			tree: []string{
				`session "api"`,
				`  ~ starting_directory: "/old" -> "/srv"`,
			},
			changes: 1,
		},
		{
			name: "a declared session directory whose live path is gone says so",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv"))),
			live: savedSession{Name: "api", StartingDirectory: "/old", DirectoryMissing: true, Windows: []savedWindow{liveWindow("edit", true, livePane("shell", "/srv", true))}},
			tree: []string{
				`session "api"`,
				`  ~ starting_directory: "/old" ` + labelNoLongerHere + ` -> "/srv"`,
			},
			changes: 1,
		},
		{
			name: "the declared window focus on another live window is a question",
			want: desired{Name: "api", Directory: "/srv", DirectoryDeclared: true, Focus: 1, Windows: []desiredWindow{wantWindow("edit", -1, wantPane("shell", "/srv")), wantWindow("run", -1, wantPane("server", "/srv"))}},
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", true)), liveWindow("run", false, livePane("server", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  ? focus: window "edit" [1], profile window "run" [2]; ` + reasonFocusWindow,
			},
			toCheck: 1,
		},
		{
			name: "a live active window that tmux named is shown with its name and position",
			want: desired{Name: "api", Directory: "/srv", DirectoryDeclared: true, Focus: 0, Windows: []desiredWindow{wantWindow("edit", -1, wantPane("shell", "/srv"))}},
			live: liveSession(liveWindow("edit", false, livePane("shell", "/srv", true)), autoWindow("zsh", true, livePane("tail", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  ? focus: window "zsh" ` + labelNamedByTmux + ` [2], profile window "edit" [1]; ` + reasonFocusWindow,
				`  - window "zsh" [2] ` + labelNamedByTmux,
				`    - pane "tail" [1]`,
			},
			removes: 2,
			toCheck: 1,
		},
		{
			name: "no declared focus gives no line whatever is active",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv")), wantWindow("run", -1, wantPane("server", "/srv"))),
			live: liveSession(liveWindow("edit", false, livePane("shell", "/srv", false)), liveWindow("run", true, livePane("server", "/srv", true))),
			tree: []string{noChanges, notComparedProvenance, notComparedGeometry},
		},
		{
			name: "the pane focus is a question per window and names an unnamed live pane at an extra position",
			want: wantSession(wantWindow("edit", 0, wantPane("shell", "/srv")), wantWindow("run", 0, wantPane("server", "/srv"))),
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", false), hostPane("/srv", true)), liveWindow("run", false, livePane("server", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  window "edit" [1]`,
				`    ? focus: pane ` + labelUnnamed + ` [2], profile pane "shell" [1]; ` + reasonFocusPane,
				`    - pane ` + labelUnnamed + ` [2]`,
			},
			removes: 1,
			toCheck: 1,
		},
		{
			name: "a pane focus on a named live pane prints its title",
			want: wantSession(wantWindow("edit", 1, wantPane("shell", "/srv"), wantPane("tests", "/srv"))),
			live: liveSession(liveWindow("edit", true, livePane("shell", "/srv", true), livePane("tests", "/srv", false))),
			tree: []string{
				`session "api"`,
				`  window "edit" [1]`,
				`    ? focus: pane "shell" [1], profile pane "tests" [2]; ` + reasonFocusPane,
			},
			toCheck: 1,
		},
		{
			name: "hostile names print quoted on one line",
			want: wantSession(wantWindow(`a"b;#c`, -1, wantPane("p", "/srv"))),
			live: liveSession(liveWindow("x\ny\x1b[31m", true, livePane("q\tr", "/srv", true))),
			tree: []string{
				`session "api"`,
				`  + window "a\"b;#c" [1]`,
				`    + pane "p" [1]  /srv`,
				`  - window "x\ny\x1b[31m" [1]`,
				`    - pane "q\tr" [1]`,
			},
			adds:    2,
			removes: 2,
		},
		{
			name: "two facts on the session count as one change",
			want: wantSession(wantWindow("edit", -1, wantPane("shell", "/srv")), wantWindow("run", -1, wantPane("server", "/srv"))),
			live: savedSession{Name: "api", StartingDirectory: "/old", Windows: []savedWindow{liveWindow("run", true, livePane("server", "/srv", true)), liveWindow("edit", false, livePane("shell", "/srv", true))}},
			tree: []string{
				`session "api"`,
				`  ~ starting_directory: "/old" -> "/srv"`,
				`  ~ order: run, edit -> edit, run`,
			},
			changes: 1,
		},
		{
			name: "a fact and a question on the session count as one change only",
			want: desired{Name: "api", Directory: "/srv", DirectoryDeclared: true, Focus: 0, Windows: []desiredWindow{wantWindow("edit", -1, wantPane("shell", "/srv")), wantWindow("run", -1, wantPane("server", "/srv"))}},
			live: savedSession{Name: "api", StartingDirectory: "/old", Windows: []savedWindow{liveWindow("edit", false, livePane("shell", "/srv", true)), liveWindow("run", true, livePane("server", "/srv", true))}},
			tree: []string{
				`session "api"`,
				`  ~ starting_directory: "/old" -> "/srv"`,
				`  ? focus: window "run" [2], profile window "edit" [1]; ` + reasonFocusWindow,
			},
			changes: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := compare(c.want, c.live)

			assert.Equal(t, c.tree, tree(t, p))
			assert.Equal(t, c.adds, p.adds, "adds")
			assert.Equal(t, c.changes, p.changes, "changes")
			assert.Equal(t, c.removes, p.removes, "removes")
			assert.Equal(t, c.toCheck, p.toCheck, "toCheck")
			assert.False(t, p.creation)
		})
	}
}

func TestCompareDirectoriesThroughSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Mkdir(real, 0o700))
	require.NoError(t, os.Symlink(real, link))

	t.Run("a pane directory equal through a symlink gives no line", func(t *testing.T) {
		want := desired{Name: "api", Directory: link, DirectoryDeclared: true, Focus: -1, Windows: []desiredWindow{wantWindow("edit", -1, wantPane("shell", link))}}
		live := savedSession{Name: "api", StartingDirectory: real, Windows: []savedWindow{liveWindow("edit", true, livePane("shell", real, true))}}

		p := compare(want, live)

		assert.Equal(t, []string{noChanges, notComparedProvenance, notComparedGeometry}, tree(t, p))
		assert.Zero(t, p.changes)
		assert.Zero(t, p.toCheck)
	})

	t.Run("a trailing slash does not make a difference", func(t *testing.T) {
		want := desired{Name: "api", Directory: real + "/", DirectoryDeclared: true, Focus: -1, Windows: []desiredWindow{wantWindow("edit", -1, wantPane("shell", real+"/"))}}
		live := savedSession{Name: "api", StartingDirectory: real, Windows: []savedWindow{liveWindow("edit", true, livePane("shell", real, true))}}

		p := compare(want, live)

		assert.Equal(t, []string{noChanges, notComparedProvenance, notComparedGeometry}, tree(t, p))
	})
}

func TestCreationPlan(t *testing.T) {
	t.Run("marks every item as an addition with its directory", func(t *testing.T) {
		want := wantSession(wantWindow("edit", 1, wantPane("shell", "/srv"), wantPane("logs", "/var/log")), wantWindow("run", -1, wantPane("server", "/srv")))

		p := creationPlan(want)

		assert.True(t, p.creation)
		assert.Equal(t, []string{
			`+ session "api"  /srv`,
			`  + window "edit" [1]`,
			`    + pane "shell" [1]  /srv`,
			`    + pane "logs" [2]  /var/log`,
			`  + window "run" [2]`,
			`    + pane "server" [1]  /srv`,
			"Plan: 6 to add, 0 to change, 0 to remove.",
			notComparedProvenance,
			notComparedGeometry,
			hintCreate,
		}, report(t, p, false))
		assert.Equal(t, 6, p.adds)
		assert.Zero(t, p.changes)
		assert.Zero(t, p.removes)
		assert.Zero(t, p.toCheck)
	})

	t.Run("names the current directory where the profile declares none", func(t *testing.T) {
		want := desired{Name: "nodir", Directory: "/home/v", Focus: -1, Windows: []desiredWindow{wantWindow("w", -1, desiredPane{Name: "p", Directory: "/home/v", InheritsSessionDirectory: true}, wantPane("q", "/var/log"))}}

		p := creationPlan(want)

		assert.Equal(t, []string{
			`+ session "nodir"  ` + labelCurrentDir,
			`  + window "w" [1]`,
			`    + pane "p" [1]  ` + labelCurrentDir,
			`    + pane "q" [2]  /var/log`,
		}, tree(t, p))
		assert.Equal(t, 4, p.adds)
	})

	t.Run("quotes the session name", func(t *testing.T) {
		p := creationPlan(desired{Name: `a"b`, Directory: "/srv", DirectoryDeclared: true, Focus: -1})

		assert.Equal(t, []string{`+ session ` + strconv.Quote(`a"b`) + `  /srv`}, tree(t, p))
		assert.Equal(t, 1, p.adds)
	})
}
