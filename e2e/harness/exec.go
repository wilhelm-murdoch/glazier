package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// DefaultTimeout is the deadline of a command without Opts.Timeout.
	DefaultTimeout = 20 * time.Second
	// killDelay is the time between SIGTERM and SIGKILL after a deadline.
	killDelay        = 2 * time.Second
	signalExitOffset = 128
	maxLoggedOutput  = 4000
)

// Opts changes how one command runs. The zero value runs in the work directory
// of the case with the environment of the case and DefaultTimeout.
type Opts struct {
	Dir     string        // relative to the work directory, or absolute
	Env     []string      // KEY=VALUE pairs on top of the environment of the case
	Stdin   string        // the standard input; empty means /dev/null
	Timeout time.Duration // the deadline; after it, SIGTERM, then SIGKILL
	Quiet   bool          // leave the command out of the log of the case
}

// Result is one finished command.
type Result struct {
	Args     []string
	Code     int // the exit code, or 128 + the signal number
	Stdout   string
	Stderr   string
	Duration time.Duration
	TimedOut bool // the command did not finish before its deadline
}

// Process is a command that runs in the background.
type Process struct {
	c      *Case
	cmd    *exec.Cmd
	cancel context.CancelFunc
	ctx    context.Context
	stdout *syncBuffer
	stderr *syncBuffer
	start  time.Time
	quiet  bool
	once   sync.Once
	result *Result
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Succeeded reports whether the command exited 0 before its deadline.
func (r *Result) Succeeded() bool { return r.Code == 0 && !r.TimedOut }

// Failed reports whether the command exited non-zero before its deadline. A
// hang is neither a success nor a failure.
func (r *Result) Failed() bool { return r.Code != 0 && !r.TimedOut }

// Output is the standard output and then the standard error.
func (r *Result) Output() string { return r.Stdout + r.Stderr }

// Start runs a command in the background in a process group of its own.
func (c *Case) Start(o Opts, name string, args ...string) *Process {
	c.t.Helper()
	timeout := o.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- the harness runs the binaries under test
	cmd.Dir = c.resolve(o.Dir)
	cmd.Env = c.environ(o.Env)
	if o.Stdin != "" {
		cmd.Stdin = strings.NewReader(o.Stdin)
	}

	p := &Process{c: c, cmd: cmd, cancel: cancel, ctx: ctx, stdout: &syncBuffer{}, stderr: &syncBuffer{}, quiet: o.Quiet}
	cmd.Stdout, cmd.Stderr = p.stdout, p.stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killGroup(cmd, syscall.SIGTERM) }
	cmd.WaitDelay = killDelay

	p.start = time.Now()
	if err := cmd.Start(); err != nil {
		cancel()
		c.t.Fatalf("start %s: %v", name, err)
	}

	c.track(cmd)
	return p
}

// Exec runs a command to the end and returns its result.
func (c *Case) Exec(o Opts, name string, args ...string) *Result {
	c.t.Helper()
	return c.Start(o, name, args...).Wait()
}

// Signal sends a signal to the process only, not to its group.
func (p *Process) Signal(sig syscall.Signal) {
	p.c.t.Helper()
	if err := p.cmd.Process.Signal(sig); err != nil {
		p.c.t.Errorf("signal %v to %d: %v", sig, p.cmd.Process.Pid, err)
	}
}

// Wait waits until the process ends or its deadline passes.
func (p *Process) Wait() *Result {
	p.c.t.Helper()
	first := false
	p.once.Do(func() {
		first = true
		err := p.cmd.Wait()
		timedOut := errors.Is(p.ctx.Err(), context.DeadlineExceeded)
		p.cancel()
		if timedOut {
			_ = killGroup(p.cmd, syscall.SIGKILL)
		}

		p.result = &Result{
			Args:     p.cmd.Args,
			Code:     exitCode(p.cmd, err),
			Stdout:   p.stdout.String(),
			Stderr:   p.stderr.String(),
			Duration: time.Since(p.start),
			TimedOut: timedOut,
		}
	})

	if first && !p.quiet {
		p.c.logResult(p.result)
	}

	return p.result
}

// Describe summarises a result for the detail of a check.
func (r *Result) Describe() string {
	if r.TimedOut {
		return fmt.Sprintf("timed out (hang) after %s", r.Duration.Round(time.Millisecond))
	}

	return fmt.Sprintf("exit %d, stdout %s, stderr %s", r.Code,
		strconv.Quote(truncate(r.Stdout, maxDetailOutput)), strconv.Quote(truncate(r.Stderr, maxDetailOutput)))
}

func (c *Case) logResult(r *Result) {
	c.t.Helper()
	state := ""
	if r.TimedOut {
		state = " (timed out)"
	}

	c.t.Logf("$ %s\nexit %d in %s%s\n--- stdout\n%s--- stderr\n%s",
		strings.Join(r.Args, " "), r.Code, r.Duration.Round(time.Millisecond), state,
		clip(r.Stdout, maxLoggedOutput), clip(r.Stderr, maxLoggedOutput))
}

func exitCode(cmd *exec.Cmd, err error) int {
	state := cmd.ProcessState
	if state == nil {
		if err != nil {
			return -1
		}

		return 0
	}

	if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return signalExitOffset + int(ws.Signal())
	}

	return state.ExitCode()
}

func killGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}

	return syscall.Kill(-cmd.Process.Pid, sig)
}

// clip shortens output for the log and ends it with a newline.
func clip(s string, n int) string {
	s = truncate(s, n)
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}

	return s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}

	return s
}

// Processes returns the command line of each process that contains every
// substring. Give the socket of the case as one substring, so that a
// parallel case cannot match.
func (c *Case) Processes(substrings ...string) []string {
	c.t.Helper()
	out, err := exec.Command("ps", "-A", "-o", "args=").Output()
	if err != nil {
		c.t.Fatalf("ps: %v", err)
	}

	var found []string
	for _, line := range strings.Split(string(out), "\n") {
		if containsAll(line, substrings) {
			found = append(found, strings.TrimSpace(line))
		}
	}

	return found
}

func containsAll(s string, substrings []string) bool {
	for _, sub := range substrings {
		if !strings.Contains(s, sub) {
			return false
		}
	}

	return true
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
