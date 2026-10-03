package harness

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	terminalRows = 40
	terminalCols = 120
)

// Terminal is a command that runs on a pseudo-terminal of 120 columns and 40
// rows, for example an attached "glaze up".
type Terminal struct {
	c        *Case
	cmd      *exec.Cmd
	pty      *os.File
	out      *syncBuffer
	start    time.Time
	timedOut atomic.Bool
	timer    *time.Timer
	exited   chan struct{}
	once     sync.Once
	result   *Result
}

// StartTerminal runs a command on a new pseudo-terminal in the background.
func (c *Case) StartTerminal(o Opts, name string, args ...string) *Terminal {
	c.t.Helper()
	cmd := exec.Command(name, args...) // #nosec G204 -- the harness runs the binaries under test
	cmd.Env = c.environ(o.Env)
	cmd.Dir = c.resolve(o.Dir)
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: terminalRows, Cols: terminalCols})
	if err != nil {
		c.t.Fatalf("start %s on a terminal: %v", name, err)
	}

	c.track(cmd)
	term := &Terminal{c: c, cmd: cmd, pty: f, out: &syncBuffer{}, start: time.Now(), exited: make(chan struct{})}
	go func() { _, _ = io.Copy(term.out, f) }()
	go func() { _ = cmd.Wait(); close(term.exited) }()
	timeout := o.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	term.timer = time.AfterFunc(timeout, func() {
		term.timedOut.Store(true)
		_ = killGroup(cmd, syscall.SIGKILL)
	})

	c.t.Cleanup(term.Close)
	return term
}

// Output is everything that the command wrote to the terminal so far.
func (term *Terminal) Output() string { return term.out.String() }

// Wait waits until the command ends or its deadline passes.
func (term *Terminal) Wait() *Result {
	term.c.t.Helper()
	<-term.exited
	first := false
	term.once.Do(func() {
		first = true
		term.timer.Stop()
		// The copy goroutine can still hold the last bytes; give it a moment.
		time.Sleep(pollInterval)
		term.result = &Result{
			Args:     term.cmd.Args,
			Code:     exitCode(term.cmd, nil),
			Stdout:   term.Output(),
			Duration: time.Since(term.start),
			TimedOut: term.timedOut.Load(),
		}
	})

	if first {
		term.c.logResult(term.result)
	}

	return term.result
}

// Close ends the command and closes the terminal.
func (term *Terminal) Close() {
	term.timer.Stop()
	_ = killGroup(term.cmd, syscall.SIGKILL)
	<-term.exited
	if err := term.pty.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		term.c.t.Logf("close the terminal: %v", err)
	}
}
