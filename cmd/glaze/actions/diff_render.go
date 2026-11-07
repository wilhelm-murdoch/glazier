package actions

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The fixed sentences of the report; the tests and the docs quote them from here.
const (
	notComparedProvenance = "Not compared: commands, envs, hooks and options. tmux does not report what glaze set."
	notComparedGeometry   = "Not compared: size, adjust and layout. tmux reports them in a form that changes with the terminal size."
	hintApply             = "Run glaze up --clear with the same profile and flags to apply the profile. The kill ends every process in the session."
	hintApplyInside       = "Run glaze up --clear with the same profile and flags from another session or outside tmux. The kill ends every process in the session, including this shell."
	hintCreate            = "Run glaze up with the same profile and flags to create the session."
	noChangesFormat       = "No changes. Session %s matches the profile in everything glaze can compare."
)

// render writes the report: the tree, the Plan line, the two fixed lines and the hint, or the no-change sentence.
func render(w io.Writer, p plan, inside bool) error {
	var b strings.Builder

	if p.adds+p.changes+p.removes+p.toCheck == 0 {
		fmt.Fprintf(&b, noChangesFormat+"\n", strconv.Quote(p.root.name))
		b.WriteString(notComparedProvenance + "\n" + notComparedGeometry + "\n")
		_, err := io.WriteString(w, b.String())

		return err
	}

	renderItem(&b, p.root, 0)

	fmt.Fprintf(&b, "Plan: %d to add, %d to change, %d to remove", p.adds, p.changes, p.removes)
	if p.toCheck > 0 {
		fmt.Fprintf(&b, ", %d to check", p.toCheck)
	}

	b.WriteString(".\n" + notComparedProvenance + "\n" + notComparedGeometry + "\n")

	switch {
	case p.creation:
		b.WriteString(hintCreate + "\n")
	case p.adds+p.changes+p.removes == 0:
	case inside:
		b.WriteString(hintApplyInside + "\n")
	default:
		b.WriteString(hintApply + "\n")
	}

	_, err := io.WriteString(w, b.String())

	return err
}

// visible reports whether an item prints: it has a mark, a line of its own, or a child that prints.
func visible(it item) bool {
	if it.mark != markSame || len(it.lines) > 0 {
		return true
	}

	for _, child := range it.children {
		if visible(child) {
			return true
		}
	}

	return false
}

// renderItem writes one item and its lines and children; a matched item with nothing to show under it is left out.
func renderItem(b *strings.Builder, it item, depth int) {
	if !visible(it) {
		return
	}

	indent := strings.Repeat("  ", depth)
	b.WriteString(indent)
	if it.mark != markSame {
		b.WriteByte(it.mark)
		b.WriteByte(' ')
	}

	b.WriteString(it.kind + " ")
	if it.name == "" && it.kind == "pane" {
		b.WriteString(labelUnnamed)
	} else {
		b.WriteString(strconv.Quote(it.name))
	}

	if it.pos > 0 {
		fmt.Fprintf(b, " [%d]", it.pos)
	}

	if it.note != "" {
		b.WriteString(" " + it.note)
	}

	if it.column != "" {
		b.WriteString("  " + it.column)
	}

	b.WriteByte('\n')

	for _, l := range it.lines {
		b.WriteString(indent + "  ")
		b.WriteByte(l.mark)
		b.WriteString(" " + l.text + "\n")
	}

	for _, child := range it.children {
		renderItem(b, child, depth+1)
	}
}
