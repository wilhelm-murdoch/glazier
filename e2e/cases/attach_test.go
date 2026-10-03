package cases

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// attachRC matches the line RC=<exit code> that a command in a pane appends to its output file.
var attachRC = regexp.MustCompile(`(?m)^RC=(\d+)$`)

// up attaches a client on a terminal, and glaze in a pane of tmux finds the right server and session.
func TestAttach(t *testing.T) {
	harness.Run(t, "attach_new", func(c *harness.Case) {
		c.Simple("at")
		c.StartTerminal(harness.Opts{}, "sh", "-c", `"$0" up --socket-name "$1"; echo RC=$? > "$2"`, c.Env().Glaze, c.Socket, c.Path("rc"))
		attachWaitClient(c, "up (attached) attaches a client", "at")
		c.Tmux("detach-client", "-s", "=at")
		c.EventuallyEqual("up exits 0 after detach", "0", func() string { return attachExitCode(c, "rc") })
	})

	harness.Run(t, "attach_existing", func(c *harness.Case) {
		c.Simple("ae")
		c.Must(c.Up(), "up")
		c.StartTerminal(harness.Opts{}, c.Env().Glaze, "up", "--socket-name", c.Socket)
		attachWaitClient(c, "up attaches to an existing session", "ae")
	})

	harness.Run(t, "attach_no_tty", func(c *harness.Case) {
		c.Simple("nt")
		r := c.Glaze("up", "--socket-name", c.Socket)
		c.Finishes(r, "up without --detached and no TTY ends")
		c.Logf("up without --detached and no TTY: rc=%d session_exists=%t windows=%s", r.Code, c.HasSession("nt"), c.WindowNames("nt"))
	})

	// glaze runs in a pane of an attached client, so ls and save see the session of that client.
	harness.Run(t, "attach_inside_tmux", func(c *harness.Case) {
		c.Simple("inner", "inner.glaze")
		c.Mkdir("hostdir")
		c.Tmux("new-session", "-d", "-s", "host", "-c", c.Path("hostdir"))
		attachTerminal(c, c.Socket, "host")
		attachWaitClient(c, "host client attached", "host")
		out, _ := attachRun(c, c.Socket, "host", "", "ls.txt", "ls", "--socket-name", c.Socket)
		c.Match("ls marks the current session with *", `host\*`, out)
		out, _ = attachRun(c, c.Socket, "host", "", "save.txt", "save", "--stdout", "--socket-name", c.Socket)
		c.Match("save inside tmux captures current session", `name += "host"`, out)
		_, rc := attachRun(c, c.Socket, "host", "", "up.txt", "up", "--socket-name", c.Socket, "--profile-path", c.Path("inner.glaze"))
		c.EventuallyEqual("up inside tmux switches the client", "inner", func() string { return attachFirstClient(c) })
		c.Equal("up inside tmux exits 0", "0", rc)
		_, rc = attachRun(c, c.Socket, "host", c.Dir, "up2.txt", "up", "--socket-name", c.Socket, "--profile-path", "inner.glaze")
		c.Equal("up inside tmux with a relative --profile-path exits 0", "0", rc)
	})

	// glaze in a pane without a socket flag uses the server of that pane.
	harness.Run(t, "attach_inside_tmux_default_server", func(c *harness.Case) {
		c.Simple("nd", "nd.glaze")
		c.Tmux("new-session", "-d", "-s", "host")
		out, _ := attachRun(c, c.Socket, "host", "", "o", "up", "--detached", "--profile-path", c.Path("nd.glaze"))
		c.True("glaze inside a pane without socket flags targets the enclosing server", c.HasSession("nd"), "session not on enclosing server; out %q", out)
	})

	// glaze runs in a pane of the host server and targets a different server.
	harness.Run(t, "attach_inside_other_server", func(c *harness.Case) {
		hostSocket := c.Socket + "h"
		c.Simple("os", "os.glaze")
		c.TmuxOn(hostSocket, "-f", "/dev/null", "new-session", "-d", "-s", "host")
		attachTerminal(c, hostSocket, "host")
		if !c.WaitUntil(harness.Patience, func() bool { return strings.TrimSpace(c.TmuxOn(hostSocket, "list-clients").Stdout) != "" }) {
			c.T().Fatal("the client of the host server did not attach")
		}

		out, rc := attachRun(c, hostSocket, "host", "", "up.txt", "up", "--socket-name", c.Socket, "--profile-path", c.Path("os.glaze"))
		c.Equal("up inside another server exits 0", "0", rc)
		c.SessionExists("up inside another server creates the session", "os")
		c.Match("up inside another server shows the attach command", "attach -t '=os'", out)
		c.Equal("up inside another server leaves the host client alone", "host", firstLine(c.TmuxOn(hostSocket, "list-clients", "-F", "#{client_session}").Stdout))
		c.Equal("up inside another server attaches no client", "", c.Tmux("list-clients"))
		out, _ = attachRun(c, hostSocket, "host", "", "ls.txt", "ls", "--socket-name", c.Socket)
		c.NoMatch("ls inside another server marks no session", `\*`, out)
		out, rc = attachRun(c, hostSocket, "host", "", "save.txt", "save", "--stdout", "--socket-name", c.Socket)
		c.Match("save inside another server asks for --session", "--session", out)
		c.Equal("save inside another server fails", strconv.Itoa(exitFailure), rc)
	})

	// up --clear inside the session that it would kill must refuse, because it would also end glaze.
	harness.Run(t, "attach_clear_inside_target", func(c *harness.Case) {
		c.Simple("self", "self.glaze")
		c.Must(c.Up("--profile-path", "self.glaze"), "up")
		attachTerminal(c, c.Socket, "self")
		attachWaitClient(c, "client attached to the target", "self")
		out, rc := attachRun(c, c.Socket, "self", "", "clear.txt", "up", "--clear", "--socket-name", c.Socket, "--profile-path", c.Path("self.glaze"))
		c.True("--clear inside the target does not end the shell that runs it", rc != "", "no exit code; out %q", out)
		c.Equal("--clear inside the target fails", strconv.Itoa(exitFailure), rc)
		c.Match("--clear inside the target says why", "would also end glaze", out)
		c.SessionExists("--clear inside the target keeps the session", "self")
	})
}

// attachTerminal attaches a tmux client on a terminal to a session of a server.
func attachTerminal(c *harness.Case, socket, session string) {
	c.StartTerminal(harness.Opts{}, c.Env().Tmux, "-L", socket, "attach", "-t", "="+session)
}

// attachWaitClient checks that the first client of the server of the case attaches to a session.
func attachWaitClient(c *harness.Case, label, session string) {
	c.EventuallyEqual(label, session, func() string { return attachFirstClient(c) })
}

// attachFirstClient returns the session of the first client of the server of the case.
func attachFirstClient(c *harness.Case) string {
	if sessions := c.ClientSessions(); len(sessions) > 0 {
		return sessions[0]
	}

	return ""
}

// attachRun types a glaze command into the first pane of a session and returns its output and exit code. The command
// runs in dir, or in the directory of the pane when dir is "", and writes both to the file out.
func attachRun(c *harness.Case, socket, session, dir, out string, args ...string) (output, rc string) {
	words := []string{harness.ShellQuote(c.Env().Glaze)}
	for _, arg := range args {
		words = append(words, harness.ShellQuote(arg))
	}

	file := harness.ShellQuote(c.Path(out))
	line := strings.Join(words, " ") + " > " + file + " 2>&1; echo RC=$? >> " + file
	if dir != "" {
		line = "cd " + harness.ShellQuote(dir) + " && " + line
	}

	c.TypeOn(socket, "="+session+":", line)
	c.WaitUntil(hangTimeout, func() bool { return attachExitCode(c, out) != "" })
	return c.Read(out), attachExitCode(c, out)
}

// attachExitCode returns the exit code in the RC= line of a file, or "" before the line is there.
func attachExitCode(c *harness.Case, rel string) string {
	if match := attachRC.FindStringSubmatch(c.Read(rel)); match != nil {
		return match[1]
	}

	return ""
}
