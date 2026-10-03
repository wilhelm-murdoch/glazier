package harness

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	terminalRows = 40
	terminalCols = 120
	closeTimeout = 5 * time.Second
)

// Control is a tmux control client (tmux -C attach). An attached client makes
// tmux behave as it does for a user: a new window starts in the session path
// and a client-attached hook fires.
type Control struct {
	c     *Case
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

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

// AttachControl attaches a control client to a session and waits until tmux
// lists it. The client stays attached until Close.
func (c *Case) AttachControl(session string) *Control {
	c.t.Helper()
	cmd := exec.Command(c.env.Tmux, "-L", c.Socket, "-C", "attach", "-t", "="+session) // #nosec G204 -- the tmux binary under test
	cmd.Env = c.environ(nil)
	cmd.Dir = c.cwd
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		c.t.Fatalf("start the control client: %v", err)
	}
	c.track(cmd)
	if !c.WaitUntil(Patience, func() bool { return len(c.lines("list-clients", "-t", "="+session)) > 0 }) {
		c.t.Errorf("the control client did not attach to %q", session)
	}
	return &Control{c: c, cmd: cmd, stdin: stdin}
}

// Send sends one tmux command through the control client.
func (cc *Control) Send(command string) {
	cc.c.t.Helper()
	if _, err := fmt.Fprintln(cc.stdin, command); err != nil {
		cc.c.t.Errorf("send %q to the control client: %v", command, err)
	}
}

// Close detaches the control client.
func (cc *Control) Close() {
	_ = cc.stdin.Close()
	done := make(chan struct{})
	go func() { _ = cc.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(closeTimeout):
		_ = killGroup(cc.cmd, syscall.SIGKILL)
		<-done
	}
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
