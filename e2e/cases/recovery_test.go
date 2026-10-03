package cases

import (
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// recoverySignals maps each signal that stops up to its name and its exit code.
var recoverySignals = []struct {
	name string
	sig  syscall.Signal
	code int
}{
	{"TERM", syscall.SIGTERM, 143},
	{"INT", syscall.SIGINT, 130},
}

// Names that tmux can confuse with another target, a retry after a failed up and a signal during up.
func TestRecovery(t *testing.T) {
	harness.Run(t, "name_collision", func(c *harness.Case) {
		c.Fixture("recovery/name-collision.glaze")
		c.OK(c.Up(), "session and first window share a name")
		c.Equal("both windows created", "dev,logs", c.WindowNames("dev"))
	})

	harness.Run(t, "name_collision_other_session", func(c *harness.Case) {
		c.Tmux("new-session", "-d", "-s", "web")
		c.Fixture("recovery/other-session-window.glaze")
		c.OK(c.Up(), "window named after another session")
		c.Equal("api windows", "web,db", c.WindowNames("api"))
		c.Equal("web session untouched", "1", fmt.Sprint(c.WindowCount("web")))
	})

	harness.Run(t, "numeric_session", func(c *harness.Case) {
		c.Fixture("recovery/numeric-name.glaze")
		for _, n := range []string{"1", "42", "0"} {
			c.KillServer()
			c.Tmux("new-session", "-d", "-s", "other")
			c.Tmux("new-window", "-t", "=other:")
			c.OK(c.Up("--var", "name="+n), fmt.Sprintf("numeric session name [%s]", n))
			c.Equal(fmt.Sprintf("numeric session [%s] windows", n), "a,b", c.WindowNames(n))
			c.Equal(fmt.Sprintf("other session untouched by numeric [%s]", n), "2", fmt.Sprint(c.WindowCount("other")))
		}
	})

	harness.Run(t, "up_prefix_existing", func(c *harness.Case) {
		c.Simple("project")
		c.Tmux("new-session", "-d", "-s", "project-long")
		c.OK(c.Up(), "up [project] while [project-long] runs")
		c.SessionExists("project created", "project")
	})

	harness.Run(t, "up_retry_after_failure", func(c *harness.Case) {
		c.Fixture("recovery/half-built.glaze")
		r := c.Up()
		c.Fails(r, "first up fails")
		c.ExitCode(r, "failed up exits 1", 1)
		c.SessionGone("failed up removes the partly built session", "half")
		c.Match("failed up says it removed the session", "removed the partly built session", r.Stderr)
		r = c.Up()
		c.True("retry after failed up reports a problem", r.Code != 0 && !r.TimedOut, "exit %d, half-built session kept: windows=[%s]", r.Code, c.WindowNames("half"))
		c.Fails(c.Up("--keep-on-failure"), "up --keep-on-failure still fails")
		c.SessionExists("up --keep-on-failure keeps the partly built session", "half")
		c.Equal("the kept session has the windows built so far", "good,bad", c.WindowNames("half"))
	})

	// A signal during a wait must stop glaze, remove the session it created and leave no waiting tmux client.
	for _, s := range recoverySignals {
		harness.Run(t, "up_signal_"+s.name, func(c *harness.Case) {
			c.Fixture("recovery/signal.glaze")
			p := c.Start(harness.Opts{}, c.Env().Glaze, "up", "--detached", "--socket-name", c.Socket)
			waiting := func() []string { return c.Processes("-L "+c.Socket+" ", "wait-for glaze-") }
			c.Eventually(fmt.Sprintf("up waits for the commands [%s]", s.name), 10*time.Second, func() (bool, string) {
				return len(waiting()) > 0, "no wait-for client"
			})
			p.Signal(s.sig)
			r := p.Wait()
			c.ExitCode(r, "up exits 128 + SIG"+s.name, s.code)
			c.SessionGone("SIG"+s.name+" removes the partly built session", "sg")
			c.Equal("SIG"+s.name+" leaves no wait-for client", "", strings.Join(waiting(), "\n"))
			c.Match("up names the signal", "glaze stopped on SIG"+s.name, r.Output())
		})
	}
}
