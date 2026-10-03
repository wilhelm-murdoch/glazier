package harness

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/report"
)

func TestChecksReportStatus(t *testing.T) {
	out := filepath.Join(t.TempDir(), "report.jsonl")
	rep, err := openReport(out, "unit")
	if err != nil {
		t.Fatal(err)
	}

	baseline := &Baseline{entries: map[string]string{
		report.ID("TestFake/case", "known bad"): "a reason",
		report.ID("TestFake/case", "fixed now"): "a stale reason",
	}}
	f := withCase(t, &Env{report: rep, baseline: baseline}, func(c *Case) {
		c.Equal("same", "a", "a")
		c.Equal("different", "a", "b")
		c.Equal("known bad", "a", "b")
		c.Equal("fixed now", "a", "a")
	})

	if err := rep.close(); err != nil {
		t.Fatal(err)
	}

	records, err := report.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]report.Status{}
	for _, r := range records {
		got[r.Check] = r.Status
		if r.Target != "unit" || r.Test != "TestFake/case" {
			t.Errorf("record %+v: wrong target or test", r)
		}
	}

	want := map[string]report.Status{"same": report.Pass, "different": report.Fail, "known bad": report.ExpectedFailure, "fixed now": report.UnexpectedPass}
	for check, status := range want {
		if got[check] != status {
			t.Errorf("%s: status %q, want %q", check, got[check], status)
		}
	}

	if len(f.errors) != 2 {
		t.Errorf("want 2 errors (a failure and an unexpected pass), got %q", f.errors)
	}

	if !strings.Contains(f.errorText(), `want "a", got "b"`) {
		t.Errorf("the failure does not show want and got: %s", f.errorText())
	}
}

func TestLabelsMustBeStable(t *testing.T) {
	for name, fn := range map[string]func(c *Case){
		"empty":     func(c *Case) { c.Equal(" ", "a", "a") },
		"duplicate": func(c *Case) { c.Equal("x", "a", "a"); c.Equal("x", "a", "a") },
		"path":      func(c *Case) { c.Equal("in "+c.Path("sub"), "a", "a") },
		"newline":   func(c *Case) { c.Equal("a\nb", "a", "a") },
		"separator": func(c *Case) { c.Equal("a"+report.Separator+"b", "a", "a") },
	} {
		t.Run(name, func(t *testing.T) {
			if f := withCase(t, nil, fn); f.fatal == "" {
				t.Error("the case did not stop")
			}
		})
	}
}

func TestOKAndFailsRejectAHang(t *testing.T) {
	hang := &Result{Code: 137, TimedOut: true}
	f := withCase(t, nil, func(c *Case) {
		if c.Fails(hang, "fails") {
			t.Error("Fails accepted a hang")
		}

		if c.OK(&Result{Code: 0, TimedOut: true}, "ok") {
			t.Error("OK accepted a hang")
		}

		if !c.Finishes(&Result{Code: 1}, "finishes") {
			t.Error("Finishes rejected a command that ended")
		}
	})

	if !strings.Contains(f.errorText(), "timed out (hang)") {
		t.Errorf("the failure does not say hang: %s", f.errorText())
	}
}

func TestEventually(t *testing.T) {
	f := withCase(t, nil, func(c *Case) {
		n := 0
		if !c.EventuallyEqual("becomes 3", "3", func() string { n++; return strconv.Itoa(n) }) {
			t.Error("EventuallyEqual gave up")
		}

		if c.Eventually("never", 3*pollInterval, func() (bool, string) { return false, "still false" }) {
			t.Error("Eventually passed a false condition")
		}
	})

	if !strings.Contains(f.errorText(), "still false") {
		t.Errorf("the failure does not show the detail: %s", f.errorText())
	}
}

func TestNormalize(t *testing.T) {
	withCase(t, nil, func(c *Case) {
		in := "2026-10-03 03:18:01 INF opened " + c.Home + "/.tmux.conf in " + c.Dir + "\n"
		if got, want := c.normalize(in), "INF opened $HOME/.tmux.conf in $WORK\n"; got != want {
			t.Errorf("normalize: %q, want %q", got, want)
		}
	})
}

func TestFiles(t *testing.T) {
	withCase(t, nil, func(c *Case) {
		c.Write("a/b.txt", "one\ntwo\n")
		if got := c.Lines("a/b.txt"); got != "one,two" {
			t.Errorf("Lines: %q", got)
		}

		if c.Read("missing") != "" || c.Exists("missing") {
			t.Error("a missing file exists")
		}

		c.Simple(`say "hi"`)
		if got := c.Read(".glaze"); !strings.Contains(got, `name = "say \"hi\""`) {
			t.Errorf("Simple does not quote the name: %s", got)
		}
	})
}
