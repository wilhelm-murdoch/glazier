package cases

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// signalExitOffset is what a shell adds to the number of the signal that ends a process.
const signalExitOffset = 128

// recoverySignals lists each signal that stops up, with the name that glaze prints.
var recoverySignals = []struct {
	name string
	sig  syscall.Signal
}{
	{"TERM", syscall.SIGTERM},
	{"INT", syscall.SIGINT},
}

// Names that tmux can confuse with another target, a retry after a failed up, a signal during up and a window that tmux renames.
func TestRecovery(t *testing.T) {
	harness.Run(t, "recovery_name_collision", func(c *harness.Case) {
		c.Fixture("recovery/name-collision.glaze")
		c.OK(c.Up(), "session and first window share a name")
		c.Equal("both windows created", "shared-name,logs", c.WindowNames("shared-name"))
	})

	harness.Run(t, "recovery_other_session_window", func(c *harness.Case) {
		c.TmuxSetup("new-session", "-d", "-s", "web")
		c.Fixture("recovery/other-session-window.glaze")
		c.OK(c.Up(), "window named after another session")
		c.Equal("api windows", "web,db", c.WindowNames("api"))
		c.Equal("web session untouched", "1", strconv.Itoa(c.WindowCount("web")))
	})

	// The other session has two windows, so a target that reaches it by mistake changes its count.
	harness.Run(t, "recovery_numeric_session", func(c *harness.Case) {
		c.Fixture("recovery/numeric-name.glaze")
		for _, name := range []string{"1", "42", "0"} {
			c.KillServer()
			c.TmuxSetup("new-session", "-d", "-s", "other")
			c.TmuxSetup("new-window", "-t", "=other:")
			c.OK(c.Up("--var", "name="+name), fmt.Sprintf("numeric session name [%s]", name))
			c.Equal(fmt.Sprintf("numeric session [%s] windows", name), "a,b", c.WindowNames(name))
			if !c.Equal(fmt.Sprintf("other session untouched by numeric [%s]", name), "2", strconv.Itoa(c.WindowCount("other"))) {
				c.Logf("sessions: %q", c.Sessions())
				c.Snapshot("other")
			}
		}
	})

	harness.Run(t, "recovery_prefix_existing", func(c *harness.Case) {
		c.Simple("project")
		c.TmuxSetup("new-session", "-d", "-s", "project-long")
		c.OK(c.Up(), "up [project] while [project-long] runs")
		c.SessionExists("project created", "project")
	})

	harness.Run(t, "recovery_retry_after_failure", func(c *harness.Case) {
		c.Fixture("recovery/half-built.glaze")
		r := c.Up()
		c.Fails(r, "first up fails")
		c.ExitCode(r, "failed up exits 1", exitFailure)
		c.SessionGone("failed up removes the partly built session", "half-built")
		c.Match("failed up says it removed the session", "removed the partly built session", r.Stderr)
		r = c.Up()
		c.True("retry after failed up reports a problem", r.Failed(),
			"exit %d, half-built session kept: windows=[%s]", r.Code, c.WindowNames("half-built"))
		c.Fails(c.Up("--keep-on-failure"), "up --keep-on-failure still fails")
		c.SessionExists("up --keep-on-failure keeps the partly built session", "half-built")
		c.Equal("the kept session has the windows built so far", "good,bad", c.WindowNames("half-built"))
	})

	// A signal during a wait must stop glaze, remove the session it created and leave no waiting tmux client.
	for _, s := range recoverySignals {
		harness.Run(t, "recovery_signal_"+s.name, func(c *harness.Case) {
			c.Fixture("recovery/signal.glaze")
			p := c.StartUp(harness.Opts{})
			waitClients := func() []string { return c.Processes("-L "+c.Socket+" ", "wait-for glaze-") }
			c.Eventually(fmt.Sprintf("up waits for the commands [%s]", s.name), hangTimeout, func() (bool, string) {
				return len(waitClients()) > 0, "no wait-for client"
			})

			p.Signal(s.sig)
			r := p.Wait()
			c.ExitCode(r, fmt.Sprintf("up exits 128 + SIG%s", s.name), signalExitOffset+int(s.sig))
			c.SessionGone(fmt.Sprintf("SIG%s removes the partly built session", s.name), "signal")
			c.EventuallyEqual(fmt.Sprintf("SIG%s leaves no wait-for client", s.name), "", func() string { return strings.Join(waitClients(), "\n") })
			c.Match("up names the signal", "glaze stopped on SIG"+s.name, r.Stderr)
			c.NoMatch("up does not blame the pane for the signal", "shell exited", r.Stderr)
		})
	}

	// up warns when tmux gives a window another name, for example with a hook in tmux.conf.
	harness.Run(t, "recovery_renamed_window", func(c *harness.Case) {
		c.TmuxConf("set-hook -g after-new-window 'rename-window renamed'\n")
		c.Fixture("recovery/rename-hook.glaze")
		r := c.Up()
		c.OK(r, "up with a tmux.conf hook that renames windows")
		c.Match("up warns that tmux renamed a window", `tmux renamed the window.*window=second.*name=renamed`, r.Stderr)
	})
}
