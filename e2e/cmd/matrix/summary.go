package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wilhelm-murdoch/glazier/e2e/report"
)

// The longest error line and check detail that the summary shows.
const (
	maxMessage = 160
	maxDetail  = 300
)

// targetRun is the result of one revision on one target.
type targetRun struct {
	target      string
	revision    string
	tmux        string
	exit        int
	records     []report.Record
	failedTests []failedTest
	log         string
	err         error
}

// counts is the number of checks for each status.
type counts map[report.Status]int

// summary compares the runs of all revisions and targets.
type summary struct {
	targets   []string
	revisions []revision
	runs      map[string]map[string]*targetRun // revision → target → run
}

func newSummary(targets []string, revisions []revision, runs []*targetRun) *summary {
	s := &summary{targets: targets, revisions: revisions, runs: map[string]map[string]*targetRun{}}
	for _, tr := range runs {
		if s.runs[tr.revision] == nil {
			s.runs[tr.revision] = map[string]*targetRun{}
		}

		s.runs[tr.revision][tr.target] = tr
	}

	return s
}

func (tr *targetRun) counts() counts {
	c := counts{}
	for _, r := range tr.records {
		c[r.Status]++
	}

	return c
}

// failures returns the ids of the checks that failed, sorted.
func (tr *targetRun) failures() []string {
	var ids []string
	for _, r := range tr.records {
		if r.Failed() {
			ids = append(ids, r.ID())
		}
	}

	slices.Sort(ids)
	return slices.Compact(ids)
}

// broken reports a run that failed outside its checks: a crash, a timeout or a
// test that the harness stopped.
func (tr *targetRun) broken() bool {
	return tr.err != nil || (tr.exit != 0 && len(tr.failures()) == 0)
}

// brief is the cell of the summary table.
func (tr *targetRun) brief() string {
	if tr.err != nil {
		return "error: " + tr.err.Error()
	}

	c := tr.counts()
	parts := []string{fmt.Sprintf("%d pass", c[report.Pass])}
	for _, st := range []report.Status{report.Fail, report.ExpectedFailure, report.UnexpectedPass} {
		if c[st] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c[st], st))
		}
	}

	if tr.broken() {
		parts = append(parts, fmt.Sprintf("exit %d", tr.exit))
	}

	return strings.Join(parts, ", ")
}

// problem says why the run of the head revision fails, or returns "".
func (s *summary) problem() string {
	var bad []string
	for _, target := range s.targets {
		tr := s.runs[headRevision][target]
		if tr == nil || tr.broken() || len(tr.failures()) > 0 {
			bad = append(bad, target)
		}
	}

	if len(bad) == 0 {
		return ""
	}

	return "failures on " + strings.Join(bad, ", ")
}

// markdown renders the table and the lists of checks, ready for a pull request.
func (s *summary) markdown() string {
	var b strings.Builder
	b.WriteString("| Target | tmux |")
	for _, rev := range s.revisions {
		fmt.Fprintf(&b, " %s (%s) |", rev.name, rev.label)
	}

	b.WriteString("\n| --- | --- |")
	for range s.revisions {
		b.WriteString(" --- |")
	}

	b.WriteString("\n")
	for _, target := range s.targets {
		tmux := ""
		if tr := s.anyRun(target); tr != nil {
			tmux = tr.tmux
		}

		fmt.Fprintf(&b, "| %s | %s |", target, tmux)
		for _, rev := range s.revisions {
			cell := "not run"
			if tr := s.runs[rev.name][target]; tr != nil {
				cell = tr.brief()
			}

			fmt.Fprintf(&b, " %s |", cell)
		}

		b.WriteString("\n")
	}

	if _, ok := s.runs[baseRevision]; ok {
		s.section(&b, "Fixed: these checks fail on base and pass on head", s.diff(baseRevision, headRevision))
		s.section(&b, "Broken: these checks pass on base and fail on head", s.diff(headRevision, baseRevision))
	}

	for _, rev := range s.revisions {
		s.failedChecks(&b, rev.name)
		s.section(&b, "Failed tests outside a check on "+rev.name, s.brokenTests(rev.name))
	}

	return b.String()
}

func (s *summary) anyRun(target string) *targetRun {
	for _, rev := range s.revisions {
		if tr := s.runs[rev.name][target]; tr != nil {
			return tr
		}
	}

	return nil
}

// failuresOf maps each failed check of a revision to the targets where it failed.
func (s *summary) failuresOf(rev string) map[string][]string {
	out := map[string][]string{}
	for _, target := range s.targets {
		if tr := s.runs[rev][target]; tr != nil {
			for _, id := range tr.failures() {
				out[id] = append(out[id], target)
			}
		}
	}

	return out
}

// diff maps each check that fails on rev a and not on rev b, target by target.
func (s *summary) diff(a, b string) map[string][]string {
	out := map[string][]string{}
	for _, target := range s.targets {
		ra, rb := s.runs[a][target], s.runs[b][target]
		if ra == nil || rb == nil {
			continue
		}

		inB := map[string]bool{}
		for _, id := range rb.failures() {
			inB[id] = true
		}

		for _, id := range ra.failures() {
			if !inB[id] {
				out[id] = append(out[id], target)
			}
		}
	}

	return out
}

// brokenTests maps each test that failed with no failed check to its targets.
func (s *summary) brokenTests(rev string) map[string][]string {
	out := map[string][]string{}
	for _, target := range s.targets {
		tr := s.runs[rev][target]
		if tr == nil {
			continue
		}

		failedChecks, failedTests := map[string]bool{}, map[string]bool{}
		for _, id := range tr.failures() {
			test, _, _ := strings.Cut(id, report.Separator)
			failedChecks[test] = true
		}

		for _, test := range tr.failedTests {
			failedTests[test.name] = true
		}

		for _, test := range tr.failedTests {
			if failedChecks[test.name] || isParentOf(test.name, failedChecks) || isParentOf(test.name, failedTests) {
				continue
			}

			key := test.name
			if test.message != "" {
				// The summary shows the key in a code span, which a backtick would end.
				key += ": " + strings.ReplaceAll(truncate(test.message, maxMessage), "`", "'")
			}

			out[key] = append(out[key], target)
		}

		if tr.err != nil {
			out["(runner) "+tr.err.Error()] = append(out["(runner) "+tr.err.Error()], target)
		}
	}

	return out
}

// isParentOf reports whether test is the parent of a test in the set. A parent
// test fails when a subtest fails, so the summary shows the subtest only.
func isParentOf(test string, failed map[string]bool) bool {
	for name := range failed {
		if strings.HasPrefix(name, test+"/") {
			return true
		}
	}

	return false
}

func (s *summary) section(b *strings.Builder, title string, items map[string][]string) {
	if len(items) == 0 {
		return
	}

	fmt.Fprintf(b, "\n%s:\n\n", title)
	for _, id := range sortedKeys(items) {
		fmt.Fprintf(b, "- `%s` (%s)\n", id, s.where(items[id]))
	}
}

// failedChecks lists each failed check of a revision with the detail of its
// first target, so that a failure in CI can be read without the logs.
func (s *summary) failedChecks(b *strings.Builder, rev string) {
	items := s.failuresOf(rev)
	if len(items) == 0 {
		return
	}

	fmt.Fprintf(b, "\nFailed checks on %s:\n\n", rev)
	for _, id := range sortedKeys(items) {
		fmt.Fprintf(b, "- `%s` (%s)\n", id, s.where(items[id]))
		if detail := s.runs[rev][items[id][0]].detail(id); detail != "" {
			fmt.Fprintf(b, "  - %s: `%s`\n", items[id][0], detail)
		}
	}
}

// detail returns the detail of a failed check on one line, short enough for
// the summary. A backtick would end the code span, so it becomes a quote.
func (tr *targetRun) detail(id string) string {
	for _, r := range tr.records {
		if r.Failed() && r.ID() == id {
			flat := strings.Join(strings.Fields(r.Detail), " ")
			return strings.ReplaceAll(truncate(flat, maxDetail), "`", "'")
		}
	}

	return ""
}

// where names the targets of an item, or says that it is on all of them.
func (s *summary) where(targets []string) string {
	if len(s.targets) > 1 && len(targets) == len(s.targets) {
		return "all targets"
	}

	return strings.Join(targets, ", ")
}

// sortedKeys returns the keys of a map in order, for a stable summary.
func sortedKeys(items map[string][]string) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}

	slices.Sort(keys)
	return keys
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}

	return s
}
