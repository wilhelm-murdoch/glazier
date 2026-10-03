package cases

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// attachRC matches the exit code line that a command typed into a pane appends to its output file.
var attachRC = regexp.MustCompile(`(?m)^RC=`)

// attachRCTimeout is how long a case waits for a command that it typed into a pane.
const attachRCTimeout = 10 * time.Second

func TestAttach(t *testing.T) {
	harness.Run(t, "attach_new", func(c *harness.Case) {
		c.Simple("at")
		c.StartTerminal(harness.Opts{}, "sh", "-c", `"$0" up --socket-name "$1"; echo GLAZE_RC=$? > "$2"`, c.Env().Glaze, c.Socket, c.Path("rc"))
		attachWaitClient(c, "up (attached) attaches a client", "at")
		c.Tmux("detach-client", "-s", "=at")
		c.EventuallyEqual("up exits 0 after detach", "GLAZE_RC=0", func() string { return strings.TrimSpace(c.Read("rc")) })
	})

	harness.Run(t, "attach_existing", func(c *harness.Case) {
		c.Simple("ae")
		c.Up()
		c.StartTerminal(harness.Opts{}, c.Env().Glaze, "up", "--socket-name", c.Socket)
		attachWaitClient(c, "up attaches to an existing session", "ae")
	})

	harness.Run(t, "attach_no_tty", func(c *harness.Case) {
		c.Simple("nt")
		r := c.Glaze("up", "--socket-name", c.Socket)
		c.Finishes(r, "up without --detached and no TTY ends")
		c.Logf("up without --detached and no TTY: rc=%d session_exists=%t windows=%s", r.Code, c.HasSession("nt"), c.WindowNames("nt"))
	})

	harness.Run(t, "inside_tmux", func(c *harness.Case) {
		c.Simple("inner", "inner.glaze")
		c.Mkdir("hostdir")
		c.Tmux("new-session", "-d", "-s", "host", "-c", c.Path("hostdir"))
		c.StartTerminal(harness.Opts{}, c.Env().Tmux, "-L", c.Socket, "attach", "-t", "=host")
		attachWaitClient(c, "host client attached", "host")
		attachType(c, c.Socket, "host", "%s ls --socket-name %s > %s 2>&1", c.Env().Glaze, c.Socket, c.Path("ls.txt"))
		c.EventuallyMatch("ls marks the current session with *", `host\*`, func() string { return c.Read("ls.txt") })
		attachType(c, c.Socket, "host", "%s save --stdout --socket-name %s > %s 2>/dev/null", c.Env().Glaze, c.Socket, c.Path("save.txt"))
		c.EventuallyMatch("save inside tmux captures current session", `name += "host"`, func() string { return c.Read("save.txt") })
		attachType(c, c.Socket, "host", "%s up --socket-name %s --profile-path %s > %s 2>&1; echo RC=$? >> %s",
			c.Env().Glaze, c.Socket, c.Path("inner.glaze"), c.Path("up.txt"), c.Path("up.txt"))
		c.EventuallyEqual("up inside tmux switches the client", "inner", func() string { return attachFirstClient(c) })
		attachWaitRC(c, "up.txt")
		c.Match("up inside tmux exits 0", `(?m)^RC=0$`, c.Read("up.txt"))
		attachType(c, c.Socket, "host", "cd %s && %s up --socket-name %s --profile-path inner.glaze > %s 2>&1; echo RC=$? >> %s",
			c.Dir, c.Env().Glaze, c.Socket, c.Path("up2.txt"), c.Path("up2.txt"))
		attachWaitRC(c, "up2.txt")
		c.Match("up inside tmux with a relative --profile-path exits 0", `(?m)^RC=0$`, c.Read("up2.txt"))
	})

	harness.Run(t, "inside_tmux_nested_default", func(c *harness.Case) {
		// glaze run inside a pane with no socket flags should target the pane's own server.
		c.Simple("nd", "nd.glaze")
		c.Tmux("new-session", "-d", "-s", "host")
		attachType(c, c.Socket, "host", "%s up --detached --profile-path %s > %s 2>&1; echo RC=$? >> %s", c.Env().Glaze, c.Path("nd.glaze"), c.Path("o"), c.Path("o"))
		attachWaitRC(c, "o")
		c.True("glaze inside a pane without socket flags targets the enclosing server", c.HasSession("nd"), "session not on enclosing server; out %q", c.Read("o"))
	})

	harness.Run(t, "inside_other_server", func(c *harness.Case) {
		// glaze runs in a pane of the host server and targets a different server.
		hs := c.Socket + "h"
		c.Simple("os", "os.glaze")
		c.TmuxOn(hs, "-f", "/dev/null", "new-session", "-d", "-s", "host")
		c.StartTerminal(harness.Opts{}, c.Env().Tmux, "-L", hs, "attach", "-t", "=host")
		c.WaitUntil(harness.Patience, func() bool { return strings.TrimSpace(c.TmuxOn(hs, "list-clients").Stdout) != "" })
		attachType(c, hs, "host", "%s up --socket-name %s --profile-path %s > %s 2>&1; echo RC=$? >> %s", c.Env().Glaze, c.Socket, c.Path("os.glaze"), c.Path("up.txt"), c.Path("up.txt"))
		attachWaitRC(c, "up.txt")
		c.Match("up inside another server exits 0", "RC=0", c.Read("up.txt"))
		c.SessionExists("up inside another server creates the session", "os")
		c.Match("up inside another server shows the attach command", "attach -t '=os'", c.Read("up.txt"))
		c.Equal("up inside another server leaves the host client alone", "host", firstLine(c.TmuxOn(hs, "list-clients", "-F", "#{client_session}").Stdout))
		c.Equal("up inside another server attaches no client", "", c.Tmux("list-clients"))
		attachType(c, hs, "host", "%s ls --socket-name %s > %s 2>&1; echo RC=$? >> %s", c.Env().Glaze, c.Socket, c.Path("ls.txt"), c.Path("ls.txt"))
		attachWaitRC(c, "ls.txt")
		c.NoMatch("ls inside another server marks no session", `\*`, c.Read("ls.txt"))
		attachType(c, hs, "host", "%s save --stdout --socket-name %s > %s 2>&1; echo RC=$? >> %s", c.Env().Glaze, c.Socket, c.Path("save.txt"), c.Path("save.txt"))
		attachWaitRC(c, "save.txt")
		c.Match("save inside another server asks for --session", "--session", c.Read("save.txt"))
		c.Match("save inside another server fails", "RC=1", c.Read("save.txt"))
	})

	harness.Run(t, "clear_inside_target", func(c *harness.Case) {
		c.Simple("self", "self.glaze")
		c.Up("--profile-path", "self.glaze")
		c.StartTerminal(harness.Opts{}, c.Env().Tmux, "-L", c.Socket, "attach", "-t", "=self")
		attachWaitClient(c, "client attached to the target", "self")
		attachType(c, c.Socket, "self", "%s up --clear --socket-name %s --profile-path %s > %s 2>&1; echo RC=$? >> %s",
			c.Env().Glaze, c.Socket, c.Path("self.glaze"), c.Path("clear.txt"), c.Path("clear.txt"))
		c.True("--clear inside the target does not end the shell that runs it", attachWaitRC(c, "clear.txt"), "no exit code; out %q", c.Read("clear.txt"))
		c.Match("--clear inside the target fails", "RC=1", c.Read("clear.txt"))
		c.Match("--clear inside the target says why", "would also end glaze", c.Read("clear.txt"))
		c.SessionExists("--clear inside the target keeps the session", "self")
	})
}

// attachWaitClient checks that the first client of the server of the case attaches to a session.
func attachWaitClient(c *harness.Case, label, session string) {
	c.EventuallyEqual(label, session, func() string { return attachFirstClient(c) })
}

// attachFirstClient returns the session of the first client of the server of the case.
func attachFirstClient(c *harness.Case) string {
	if got := c.ClientSessions(); len(got) > 0 {
		return got[0]
	}
	return ""
}

// attachType types a shell command line into the first pane of a session on a server and presses Enter.
// Each argument is quoted for the shell, so the format holds the shell syntax only.
func attachType(c *harness.Case, socket, session, format string, args ...string) {
	quoted := make([]any, len(args))
	for i, a := range args {
		quoted[i] = harness.ShellQuote(a)
	}
	c.TypeOn(socket, "="+session+":", fmt.Sprintf(format, quoted...))
}

// attachWaitRC waits until a file in the work directory has an RC= line. It is not a check.
func attachWaitRC(c *harness.Case, rel string) bool {
	return c.WaitUntil(attachRCTimeout, func() bool { return attachRC.MatchString(c.Read(rel)) })
}
