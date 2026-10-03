package harness

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
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

// Pid is the process id, which is also the id of its process group.
func (p *Process) Pid() int { return p.cmd.Process.Pid }

// Signal sends a signal to the process only, not to its group.
func (p *Process) Signal(sig syscall.Signal) {
	p.c.t.Helper()
	if err := p.cmd.Process.Signal(sig); err != nil {
		p.c.t.Errorf("signal %v to %d: %v", sig, p.Pid(), err)
	}
}

// Stdout is the standard output so far.
func (p *Process) Stdout() string { return p.stdout.String() }

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

func clip(s string, n int) string {
	if len(s) > n {
		s = s[:n] + "…\n"
	}
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
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
