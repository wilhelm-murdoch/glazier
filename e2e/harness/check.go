package harness

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/report"
)

// Patience is the default time that Eventually waits for a condition.
const Patience = 5 * time.Second

const (
	pollInterval    = 50 * time.Millisecond
	maxDetailOutput = 300
)

// Every check takes a label first. The label names the check in the report and
// in expected-failures.txt, so it must be unique in its case and the same on
// every target: never put a path, a pid, a time or a tmux version in it.

// OK checks that a command exits 0.
func (c *Case) OK(r *Result, label string) bool {
	c.t.Helper()
	return c.record(label+" rc=0", r.Code == 0 && !r.TimedOut, describe(r))
}

// Fails checks that a command exits non-zero, before its deadline.
func (c *Case) Fails(r *Result, label string) bool {
	c.t.Helper()
	return c.record(label+" rc!=0", r.Code != 0 && !r.TimedOut, describe(r))
}

// ExitCode checks the exact exit code of a command.
func (c *Case) ExitCode(r *Result, label string, want int) bool {
	c.t.Helper()
	return c.record(label, r.Code == want && !r.TimedOut, fmt.Sprintf("want exit %d; %s", want, describe(r)))
}

// Finishes checks that a command ends before its deadline, whatever its exit code.
func (c *Case) Finishes(r *Result, label string) bool {
	c.t.Helper()
	return c.record(label, !r.TimedOut, describe(r))
}

// Within checks that a command ends in less than d.
func (c *Case) Within(r *Result, label string, d time.Duration) bool {
	c.t.Helper()
	return c.record(label, !r.TimedOut && r.Duration < d, fmt.Sprintf("took %s, limit %s", r.Duration.Round(time.Millisecond), d))
}

// AtLeast checks that a command takes d or longer.
func (c *Case) AtLeast(r *Result, label string, d time.Duration) bool {
	c.t.Helper()
	return c.record(label, r.Duration >= d, fmt.Sprintf("took %s, minimum %s", r.Duration.Round(time.Millisecond), d))
}

// Equal checks that got is want.
func (c *Case) Equal(label, want, got string) bool {
	c.t.Helper()
	return c.record(label, got == want, fmt.Sprintf("want %s, got %s", strconv.Quote(want), strconv.Quote(got)))
}

// Match checks that s matches a regular expression.
func (c *Case) Match(label, pattern, s string) bool {
	c.t.Helper()
	return c.record(label, c.compile(pattern).MatchString(s), fmt.Sprintf("want /%s/ in %s", pattern, strconv.Quote(s)))
}

// NoMatch checks that s does not match a regular expression.
func (c *Case) NoMatch(label, pattern, s string) bool {
	c.t.Helper()
	return c.record(label, !c.compile(pattern).MatchString(s), fmt.Sprintf("unexpected /%s/ in %s", pattern, strconv.Quote(s)))
}

// True checks a condition. The detail explains a failure.
func (c *Case) True(label string, cond bool, detail string, args ...any) bool {
	c.t.Helper()
	return c.record(label, cond, fmt.Sprintf(detail, args...))
}

// SessionExists checks that the server of the case has a session.
func (c *Case) SessionExists(label, session string) bool {
	c.t.Helper()
	return c.record(label, c.HasSession(session), fmt.Sprintf("no session %s; sessions %q", strconv.Quote(session), c.Sessions()))
}

// SessionGone checks that the server of the case has no such session.
func (c *Case) SessionGone(label, session string) bool {
	c.t.Helper()
	return c.record(label, !c.HasSession(session), fmt.Sprintf("session %s is still there", strconv.Quote(session)))
}

// NoServer checks that no tmux server runs on the socket of the case.
func (c *Case) NoServer(label string) bool {
	c.t.Helper()
	return c.record(label+" no tmux server started", !c.ServerRunning(), fmt.Sprintf("a server runs with sessions %q", c.Sessions()))
}

// Eventually checks that cond becomes true within timeout. cond returns a
// detail for the failure message.
func (c *Case) Eventually(label string, timeout time.Duration, cond func() (bool, string)) bool {
	c.t.Helper()
	var detail string
	ok := c.WaitUntil(timeout, func() bool {
		var done bool
		done, detail = cond()
		return done
	})
	return c.record(label, ok, fmt.Sprintf("not true after %s: %s", timeout, detail))
}

// EventuallyEqual checks that got() becomes want within Patience.
func (c *Case) EventuallyEqual(label, want string, got func() string) bool {
	c.t.Helper()
	return c.Eventually(label, Patience, func() (bool, string) {
		g := got()
		return g == want, fmt.Sprintf("want %s, got %s", strconv.Quote(want), strconv.Quote(g))
	})
}

// EventuallyMatch checks that got() matches a regular expression within Patience.
func (c *Case) EventuallyMatch(label, pattern string, got func() string) bool {
	c.t.Helper()
	re := c.compile(pattern)
	return c.Eventually(label, Patience, func() (bool, string) {
		g := got()
		return re.MatchString(g), fmt.Sprintf("want /%s/ in %s", pattern, strconv.Quote(g))
	})
}

// WaitUntil polls cond until it is true or timeout passes. It is not a check.
func (c *Case) WaitUntil(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

// WaitFile waits until a file exists and is not empty. It is not a check.
func (c *Case) WaitFile(rel string, timeout time.Duration) bool {
	return c.WaitUntil(timeout, func() bool { return c.Read(rel) != "" })
}

func (c *Case) record(label string, pass bool, detail string) bool {
	c.t.Helper()
	c.validateLabel(label)
	id := report.ID(c.t.Name(), label)
	reason, expected := c.env.baseline.Expected(id)
	var status report.Status
	switch {
	case pass && expected:
		status = report.UnexpectedPass
		c.t.Errorf("%s: passes, but expected-failures.txt says it fails (%s); remove the entry", label, reason)
	case pass:
		status = report.Pass
	case expected:
		status = report.ExpectedFailure
		c.t.Logf("%s: expected failure (%s): %s", label, reason, detail)
	default:
		status = report.Fail
		c.t.Errorf("%s: %s", label, detail)
	}
	if status == report.Pass {
		detail = ""
	}
	c.env.report.write(c.t.Name(), label, status, detail)
	return pass
}

func (c *Case) validateLabel(label string) {
	c.t.Helper()
	switch {
	case strings.TrimSpace(label) == "":
		c.t.Fatal("a check needs a label")
	case strings.Contains(label, c.Dir), strings.Contains(label, c.tmpdir):
		c.t.Fatalf("the label %q contains a path of the run; labels must be the same on every run", label)
	case strings.ContainsAny(label, "\n\t"):
		c.t.Fatalf("the label %q contains a newline or a tab", label)
	case strings.Contains(label, report.Separator):
		c.t.Fatalf("the label %q contains %q, the separator of expected-failures.txt", label, report.Separator)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.labels[label] {
		c.t.Fatalf("the label %q is used twice in this case", label)
	}
	c.labels[label] = true
}

func (c *Case) compile(pattern string) *regexp.Regexp {
	c.t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		c.t.Fatalf("pattern /%s/: %v", pattern, err)
	}
	return re
}

// Describe summarises a result for the detail of a check.
func (r *Result) Describe() string { return describe(r) }

func describe(r *Result) string {
	if r.TimedOut {
		return fmt.Sprintf("timed out (hang) after %s", r.Duration.Round(time.Millisecond))
	}
	return fmt.Sprintf("exit %d, stdout %s, stderr %s", r.Code,
		strconv.Quote(truncate(r.Stdout, maxDetailOutput)), strconv.Quote(truncate(r.Stderr, maxDetailOutput)))
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
