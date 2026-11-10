package actions

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fatih/color"

	"github.com/wilhelm-murdoch/glazier/internal/term"
)

// The fixed sentences of the report; the tests and the docs quote them from here.
const (
	notComparedProvenance = "Not compared: commands, envs, hooks and options. tmux does not report what glaze set."
	notComparedGeometry   = "Not compared: size, adjust and layout. tmux reports them in a form that changes with the terminal size."
	hintApply             = "Run glaze up --clear with the same profile and flags to apply the profile. The kill ends every process in the session."
	hintApplyInside       = "Run glaze up --clear with the same profile and flags from another session or outside tmux. The kill ends every process in the session, including this shell."
	hintCreate            = "Run glaze up with the same profile and flags to create the session."
	noChangesLead         = "No changes."
	noChangesFormat       = noChangesLead + " Session %s matches the profile in everything glaze can compare."
)

// markColours are the colours of the marks on a terminal, as Terraform shows a plan.
var markColours = map[byte]color.Attribute{
	markAdd:    color.FgGreen,
	markRemove: color.FgRed,
	markChange: color.FgYellow,
	markCheck:  color.FgCyan,
}

// painter colours the parts of the report when it writes to a terminal.
type painter bool

// line colours a line of the tree by its mark; a line without a mark stays plain.
func (p painter) line(mark byte, s string) string {
	if attr, ok := markColours[mark]; ok {
		return term.Paint(bool(p), s, attr)
	}

	return s
}

// count colours one part of the Plan line by its mark when it counts something.
func (p painter) count(n int, what string, mark byte) string {
	s := fmt.Sprintf("%d %s", n, what)
	if n == 0 {
		return s
	}

	return p.line(mark, s)
}

// render writes the report: the tree, the Plan line, the two fixed lines and the hint, or the no-change sentence.
// With colour, it writes the same text with colour codes, so the text without them is the same report.
func render(w io.Writer, p plan, inside, colour bool) error {
	var b strings.Builder
	paint := painter(colour)
	notCompared := term.Paint(colour, notComparedProvenance, color.Faint) + "\n" + term.Paint(colour, notComparedGeometry, color.Faint) + "\n"

	if p.adds+p.changes+p.removes+p.toCheck == 0 {
		sentence := fmt.Sprintf(noChangesFormat, strconv.Quote(p.root.name))
		b.WriteString(term.Paint(colour, noChangesLead, color.Bold, color.FgGreen) + strings.TrimPrefix(sentence, noChangesLead) + "\n")
		b.WriteString(notCompared)
		_, err := io.WriteString(w, b.String())

		return err
	}

	renderItem(&b, p.root, 0, paint)

	b.WriteString(term.Paint(colour, "Plan:", color.Bold) + " " + paint.count(p.adds, "to add", markAdd) + ", " + paint.count(p.changes, "to change", markChange) + ", " + paint.count(p.removes, "to remove", markRemove))
	if p.toCheck > 0 {
		b.WriteString(", " + paint.count(p.toCheck, "to check", markCheck))
	}

	b.WriteString(".\n" + notCompared)

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
func renderItem(b *strings.Builder, it item, depth int, paint painter) {
	if !visible(it) {
		return
	}

	var line strings.Builder
	if it.mark != markSame {
		line.WriteByte(it.mark)
		line.WriteByte(' ')
	}

	line.WriteString(it.kind + " ")
	if it.name == "" && it.kind == "pane" {
		line.WriteString(labelUnnamed)
	} else {
		line.WriteString(strconv.Quote(it.name))
	}

	if it.pos > 0 {
		fmt.Fprintf(&line, " [%d]", it.pos)
	}

	if it.note != "" {
		line.WriteString(" " + it.note)
	}

	if it.column != "" {
		line.WriteString("  " + it.column)
	}

	indent := strings.Repeat("  ", depth)
	b.WriteString(indent + paint.line(it.mark, line.String()) + "\n")

	for _, l := range it.lines {
		b.WriteString(indent + "  " + paint.line(l.mark, string(l.mark)+" "+l.text) + "\n")
	}

	for _, child := range it.children {
		renderItem(b, child, depth+1, paint)
	}
}
