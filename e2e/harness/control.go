package harness

import (
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// closeTimeout is the time that Close gives the client to detach.
const closeTimeout = 5 * time.Second

// Control is a tmux control client (tmux -C attach). An attached client makes
// tmux behave as it does for a user: a new window starts in the session path
// and a client-attached hook fires.
type Control struct {
	c     *Case
	cmd   *exec.Cmd
	stdin io.WriteCloser
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
