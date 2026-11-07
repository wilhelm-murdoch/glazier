package actions

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// The marks of a plan line.
const (
	markSame   byte = 0
	markAdd    byte = '+'
	markRemove byte = '-'
	markChange byte = '~'
	markCheck  byte = '?'
)

// The reasons that end a line glaze cannot state as a fact.
const (
	reasonFocusWindow = "a user can select another window"
	reasonFocusPane   = "a user can select another pane"
	reasonDirectory   = "tmux reports where the shell is now, not where the pane started"
	reasonTitle       = "a program in the pane can set the title"
	reasonReplaced    = "glaze titles every pane it creates, so this pane is not from the profile"
)

// The unquoted tokens of the report.
const (
	labelUnnamed      = "(unnamed)"
	labelNamedByTmux  = "(named by tmux)"
	labelNoLongerHere = "(no longer exists)"
	labelCurrentDir   = "(current directory)"
)

// plan is the report of a diff: the tree and the counts of its Plan line.
type plan struct {
	root     item
	creation bool

	adds, changes, removes, toCheck int
}

// item is a session, a window or a pane in the report, with its own lines and its children.
type item struct {
	mark     byte
	kind     string
	name     string
	pos      int
	note     string
	column   string
	lines    []line
	children []item
}

// line is one attribute line under an item.
type line struct {
	mark byte
	text string
}

// compare builds the plan from the profile to the live session: what up --clear would change.
func compare(want desired, live savedSession) plan {
	p := plan{root: item{kind: "session", name: want.Name}}
	root := &p.root

	// The session directory is a fact when the profile declares it, and a question when up would use the current directory.
	if want.DirectoryDeclared {
		if !sameDirectory(live.StartingDirectory, want.Directory) {
			root.lines = append(root.lines, line{markChange, fmt.Sprintf("starting_directory: %s -> %s", livePath(live.StartingDirectory, live.DirectoryMissing), strconv.Quote(want.Directory))})
		}
	} else if !sameDirectory(live.StartingDirectory, want.Directory) {
		root.lines = append(root.lines, line{markCheck, fmt.Sprintf("starting_directory: %s, profile declares none; up would use the current directory %s", livePath(live.StartingDirectory, live.DirectoryMissing), strconv.Quote(want.Directory))})
	}

	matched := matchWindows(want, live)

	if order := orderLine(want, live, matched); order != "" {
		root.lines = append(root.lines, line{markChange, order})
	}

	if focus := windowFocusLine(want, live, matched); focus != "" {
		root.lines = append(root.lines, line{markCheck, focus})
	}

	for i, dw := range want.Windows {
		j, ok := matched[i]
		if !ok {
			root.children = append(root.children, addedWindow(dw, i+1, want))
			continue
		}

		root.children = append(root.children, compareWindow(dw, i+1, live.Windows[j], live, want))
	}

	used := map[int]bool{}
	for _, j := range matched {
		used[j] = true
	}

	for j, lw := range live.Windows {
		if !used[j] {
			root.children = append(root.children, removedWindow(lw, j+1))
		}
	}

	count(&p, *root)

	return p
}

// matchWindows pairs each profile window with the first free live window of the same name, or of tmux's naming when the profile asks for it.
func matchWindows(want desired, live savedSession) map[int]int {
	matched := map[int]int{}
	used := map[int]bool{}
	for i, dw := range want.Windows {
		for j, lw := range live.Windows {
			if used[j] {
				continue
			}

			if (dw.AutomaticDeclared && lw.Automatic) || (!dw.AutomaticDeclared && !lw.Automatic && lw.LiveName == dw.Name) {
				matched[i] = j
				used[j] = true
				break
			}
		}
	}

	return matched
}

// orderLine says when the matched windows stand in another order live than in the profile.
func orderLine(want desired, live savedSession, matched map[int]int) string {
	var profileOrder, liveOrder []string
	last := -1
	moved := false
	type pair struct {
		live int
		name string
	}

	var pairs []pair
	for i, dw := range want.Windows {
		j, ok := matched[i]
		if !ok {
			continue
		}

		if j < last {
			moved = true
		}

		last = j
		profileOrder = append(profileOrder, dw.Name)
		pairs = append(pairs, pair{j, dw.Name})
	}

	if !moved {
		return ""
	}

	for len(pairs) > 0 {
		best := 0
		for k := range pairs {
			if pairs[k].live < pairs[best].live {
				best = k
			}
		}

		liveOrder = append(liveOrder, pairs[best].name)
		pairs = append(pairs[:best], pairs[best+1:]...)
	}

	return fmt.Sprintf("order: %s -> %s", strings.Join(liveOrder, ", "), strings.Join(profileOrder, ", "))
}

// windowFocusLine says when another window than the one the profile focuses is active; tmux cannot tell who selected it.
func windowFocusLine(want desired, live savedSession, matched map[int]int) string {
	if want.Focus < 0 {
		return ""
	}

	active := -1
	for j, lw := range live.Windows {
		if lw.Focus {
			active = j
		}
	}

	if j, ok := matched[want.Focus]; ok && j == active {
		return ""
	}

	liveSide := "none"
	if active >= 0 {
		liveSide = "window " + liveWindowName(live.Windows[active]) + " [" + strconv.Itoa(active+1) + "]"
	}

	return fmt.Sprintf("focus: %s, profile window %s [%d]; %s", liveSide, strconv.Quote(want.Windows[want.Focus].Name), want.Focus+1, reasonFocusWindow)
}

// compareWindow builds the item of a matched window: the pane focus, then the panes by position.
func compareWindow(dw desiredWindow, pos int, lw savedWindow, live savedSession, want desired) item {
	it := item{kind: "window", name: dw.Name, pos: pos}

	if dw.Focus >= 0 {
		active := -1
		for k, lp := range lw.Panes {
			if lp.Focus {
				active = k
			}
		}

		if active != dw.Focus {
			liveSide := "none"
			if active >= 0 {
				liveSide = "pane " + livePaneName(lw.Panes[active]) + " [" + strconv.Itoa(active+1) + "]"
			}

			it.lines = append(it.lines, line{markCheck, fmt.Sprintf("focus: %s, profile pane %s [%d]; %s", liveSide, strconv.Quote(dw.Panes[dw.Focus].Name), dw.Focus+1, reasonFocusPane)})
		}
	}

	for k := 0; k < len(dw.Panes) || k < len(lw.Panes); k++ {
		switch {
		case k >= len(lw.Panes):
			it.children = append(it.children, item{mark: markAdd, kind: "pane", name: dw.Panes[k].Name, pos: k + 1, column: paneColumnDesired(dw.Panes[k], want)})
		case k >= len(dw.Panes):
			it.children = append(it.children, removedPane(lw.Panes[k], k+1))
		default:
			it.children = append(it.children, comparePane(dw.Panes[k], k+1, lw.Panes[k], live))
		}
	}

	return it
}

// comparePane builds the item of a matched pane: a replaced pane is a fact, a title or a directory is a question.
func comparePane(dp desiredPane, pos int, lp savedPane, live savedSession) item {
	it := item{kind: "pane", name: dp.Name, pos: pos}

	switch {
	case lp.HostTitle && lp.Title != dp.Name:
		it.lines = append(it.lines, line{markChange, fmt.Sprintf("title: %s -> %s; %s", labelUnnamed, strconv.Quote(dp.Name), reasonReplaced)})
	case lp.Title != dp.Name:
		it.lines = append(it.lines, line{markCheck, fmt.Sprintf("title: %s, profile name %s; %s", strconv.Quote(lp.Title), strconv.Quote(dp.Name), reasonTitle)})
	}

	// A pane that inherits an undeclared session directory is compared with where the session started, not with the directory of diff.
	if !lp.DirectoryUnknown {
		against := dp.Directory
		if dp.InheritsSessionDirectory {
			against = live.StartingDirectory
		}

		if !sameDirectory(lp.StartingDirectory, against) {
			it.lines = append(it.lines, line{markCheck, fmt.Sprintf("directory: %s, profile %s; %s", livePath(lp.StartingDirectory, lp.DirectoryMissing), strconv.Quote(against), reasonDirectory)})
		}
	}

	return it
}

// addedWindow is a profile window with no live match, with every pane as an addition.
func addedWindow(dw desiredWindow, pos int, want desired) item {
	it := item{mark: markAdd, kind: "window", name: dw.Name, pos: pos}
	for k, dp := range dw.Panes {
		it.children = append(it.children, item{mark: markAdd, kind: "pane", name: dp.Name, pos: k + 1, column: paneColumnDesired(dp, want)})
	}

	return it
}

// removedWindow is a live window with no profile match, with every pane as a removal.
func removedWindow(lw savedWindow, pos int) item {
	it := item{mark: markRemove, kind: "window", name: lw.LiveName, pos: pos}
	if lw.Automatic {
		it.note = labelNamedByTmux
	}

	for k, lp := range lw.Panes {
		it.children = append(it.children, removedPane(lp, k+1))
	}

	return it
}

// removedPane is a live pane with no profile position.
func removedPane(lp savedPane, pos int) item {
	it := item{mark: markRemove, kind: "pane", name: lp.Title, pos: pos}
	if lp.HostTitle {
		it.name = ""
	}

	return it
}

// creationPlan is the plan for a session that does not run: everything is to add.
func creationPlan(want desired) plan {
	p := plan{creation: true, root: item{mark: markAdd, kind: "session", name: want.Name, column: want.Directory}}
	if !want.DirectoryDeclared {
		p.root.column = labelCurrentDir
	}

	for i, dw := range want.Windows {
		p.root.children = append(p.root.children, addedWindow(dw, i+1, want))
	}

	count(&p, p.root)

	return p
}

// paneColumnDesired is the second column of an added pane: its directory, or the token for an undeclared session directory that up would fill from where it runs.
func paneColumnDesired(dp desiredPane, want desired) string {
	if dp.InheritsSessionDirectory && !want.DirectoryDeclared {
		return labelCurrentDir
	}

	return dp.Directory
}

// count fills the Plan line: a + or - item counts with each child, an item with a ~ line counts once as changed, one with only ? lines once to check.
func count(p *plan, it item) {
	switch it.mark {
	case markAdd:
		p.adds++
	case markRemove:
		p.removes++
	default:
		changed, check := false, false
		for _, l := range it.lines {
			switch l.mark {
			case markChange:
				changed = true
			case markCheck:
				check = true
			}
		}

		if changed {
			p.changes++
		} else if check {
			p.toCheck++
		}
	}

	for _, child := range it.children {
		count(p, child)
	}
}

// liveWindowName prints a live window as the report shows it.
func liveWindowName(lw savedWindow) string {
	if lw.Automatic {
		return strconv.Quote(lw.LiveName) + " " + labelNamedByTmux
	}

	return strconv.Quote(lw.LiveName)
}

// livePaneName prints a live pane as the report shows it.
func livePaneName(lp savedPane) string {
	if lp.HostTitle {
		return labelUnnamed
	}

	return strconv.Quote(lp.Title)
}

// livePath prints a directory that tmux reported, with a token when it no longer exists.
func livePath(dir string, missing bool) string {
	if missing {
		return strconv.Quote(dir) + " " + labelNoLongerHere
	}

	return strconv.Quote(dir)
}

// sameDirectory compares two paths after Clean and, where the path exists, EvalSymlinks, so /tmp and /private/tmp compare equal.
func sameDirectory(a, b string) bool {
	return canonical(a) == canonical(b)
}

func canonical(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}

	return path
}
