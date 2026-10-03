package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/report"
)

func rec(test, check string, st report.Status) report.Record {
	return report.Record{Test: test, Check: check, Status: st}
}

func TestSummaryComparesRevisions(t *testing.T) {
	targets := []string{"a", "b"}
	revisions := []revision{{name: baseRevision, label: "main"}, {name: headRevision, label: "working tree"}}
	runs := []*targetRun{
		{target: "a", revision: baseRevision, tmux: "3.3a", exit: 1, records: []report.Record{rec("TestX/c", "fixed", report.Fail), rec("TestX/c", "same", report.Pass)}},
		{target: "b", revision: baseRevision, exit: 1, records: []report.Record{rec("TestX/c", "fixed", report.Fail), rec("TestX/c", "same", report.Pass)}},
		{target: "a", revision: headRevision, exit: 1, records: []report.Record{rec("TestX/c", "fixed", report.Pass), rec("TestX/c", "same", report.Fail)}},
		{target: "b", revision: headRevision, records: []report.Record{rec("TestX/c", "fixed", report.Pass), rec("TestX/c", "same", report.Pass)}},
	}

	s := newSummary(targets, revisions, runs)
	md := s.markdown()
	for _, want := range []string{
		"| Target | tmux | base (main) | head (working tree) |",
		"| a | 3.3a | 1 pass, 1 fail | 1 pass, 1 fail |",
		"| b |  | 1 pass, 1 fail | 2 pass |",
		"Fixed: these checks fail on base and pass on head:\n\n- `TestX/c | fixed` (all targets)",
		"Broken: these checks pass on base and fail on head:\n\n- `TestX/c | same` (a)",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}

	if got := s.problem(); got != "failures on a" {
		t.Errorf("problem %q", got)
	}
}

func TestSummaryShowsFailuresOutsideChecks(t *testing.T) {
	runs := []*targetRun{
		{target: "a", revision: headRevision, exit: 1, failedTests: []failedTest{{name: "TestY"}, {name: "TestY/crash", message: "y_test.go:3: boom"}}, records: []report.Record{rec("TestX/c", "ok", report.Pass)}},
		{target: "b", revision: headRevision, err: errors.New("docker died")},
	}

	s := newSummary([]string{"a", "b"}, []revision{{name: headRevision, label: "wt"}}, runs)
	md := s.markdown()
	for _, want := range []string{"1 pass, exit 1", "error: docker died", "- `TestY/crash: y_test.go:3: boom` (a)", "(runner) docker died"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}

	if strings.Contains(md, "- `TestY` (a)") {
		t.Errorf("the parent of a failed test is listed:\n%s", md)
	}

	if s.problem() != "failures on a, b" {
		t.Errorf("problem %q", s.problem())
	}
}

func TestSummaryExpectedFailuresPass(t *testing.T) {
	runs := []*targetRun{{target: "a", revision: headRevision, records: []report.Record{rec("TestX/c", "known", report.ExpectedFailure)}}}
	s := newSummary([]string{"a"}, []revision{{name: headRevision}}, runs)
	if s.problem() != "" {
		t.Errorf("an expected failure fails the run: %q", s.problem())
	}

	if !strings.Contains(s.markdown(), "0 pass, 1 xfail") {
		t.Errorf("markdown:\n%s", s.markdown())
	}
}

func TestParseFlags(t *testing.T) {
	cfg, err := parseFlags([]string{"-targets", "alpine, tmux37", "-base-glaze", "/x"})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(cfg.targets, ",") != "alpine,tmux37" || cfg.base == "" {
		t.Errorf("config %+v", cfg)
	}

	for _, bad := range [][]string{{"-targets", "nope"}, {"-targets", ","}, {"-parallel", "0"}, {"-count", "0"}, {"extra"}} {
		if _, err := parseFlags(bad); err == nil {
			t.Errorf("%v: no error", bad)
		}
	}
}

func TestParseFailedTests(t *testing.T) {
	log := t.TempDir() + "/t.log"
	content := strings.Join([]string{
		"=== RUN   TestA/x",
		"    a_test.go:5: $ glaze up",
		"    a_test.go:9: setup failed",
		"    a_test.go:10: later line",
		"    --- FAIL: TestA/x (0.10s)",
		"--- FAIL: TestA (0.20s)",
		"--- PASS: TestB (0.00s)",
	}, "\n")
	if err := writeFile(log, content); err != nil {
		t.Fatal(err)
	}

	got := parseFailedTests(log)
	if len(got) != 2 || got[0] != (failedTest{name: "TestA/x", message: "a_test.go:9: setup failed"}) || got[1].name != "TestA" {
		t.Errorf("parseFailedTests %+v", got)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
