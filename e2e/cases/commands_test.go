package cases

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

const (
	// serialMinimum is the shortest time of an up with serial.glaze, which sleeps 2 s; 100 ms allows for timer skew.
	serialMinimum = 1900 * time.Millisecond
	// quickUpLimit is the longest time of an up that does not wait for a command.
	quickUpLimit = 5 * time.Second
	// blockProbe is long enough to show that up waits for a command and short enough to keep the case fast.
	blockProbe = 3 * time.Second
	// shellsTimeout is longer than hangTimeout, because shells.glaze runs six commands in a shell that starts for the first time.
	shellsTimeout = 15 * time.Second
	// longCommandLength is longer than the line limits of the shells: 2 KB in ash and 4 KB in dash.
	longCommandLength = 5000
)

// TestCommands checks how glaze runs pane and session commands and when it waits for them.
func TestCommands(t *testing.T) {
	harness.Run(t, "cmd_serial", func(c *harness.Case) {
		c.Fixture("commands/serial.glaze")
		r := c.Up()
		c.OK(r, "up")
		c.AtLeast(r, "up waits for non-final commands", serialMinimum)
		c.EventuallyEqual("commands ran in order", "a,b,c", fileLines(c, "o"))
		c.EventuallyEqual("no glaze buffer is left", "", pasteBuffers(c))
	})

	harness.Run(t, "cmd_last_long_running", func(c *harness.Case) {
		c.Fixture("commands/last-long-running.glaze")
		r := c.UpWith(harness.Opts{Timeout: hangTimeout})
		c.OK(r, "up with long-running final command")
		c.Within(r, "final command is fire-and-forget", quickUpLimit)
	})

	harness.Run(t, "cmd_single_long_running", func(c *harness.Case) {
		c.Fixture("commands/single-long-running.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "up with single long-running command")
	})

	// glaze has no default limit for a command, so it waits until the deadline of the case.
	harness.Run(t, "cmd_nonfinal_long_running", func(c *harness.Case) {
		c.Fixture("commands/nonfinal-long-running.glaze")
		r := c.UpWith(harness.Opts{Timeout: blockProbe})
		c.True("non-final long-running command blocks up (by design)", r.TimedOut, "%s", r.Describe())
	})

	harness.Run(t, "cmd_failing", func(c *harness.Case) {
		c.Fixture("commands/failing.glaze")
		c.OK(c.Up(), "up with a failing command")
		c.EventuallyEqual("later commands still run", "after", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_exit_then_split", func(c *harness.Case) {
		c.Fixture("commands/exit-then-split.glaze")
		c.OK(c.Up(), "a pane whose command exits does not break the next split")
		c.EventuallyEqual("the other panes keep their order", "shell,shell2", func() string { return c.PaneTitles("=exit-then-split:w") })
	})

	harness.Run(t, "cmd_trailing_comment", func(c *harness.Case) {
		c.Fixture("commands/trailing-comment.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "non-final command with trailing # comment does not hang")
		c.EventuallyEqual("both commands ran", "a,b", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_trailing_ampersand", func(c *harness.Case) {
		c.Fixture("commands/trailing-ampersand.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "non-final backgrounded command (&) does not hang")
		c.EventuallyEqual("command after & ran", "b", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_trailing_semicolon", func(c *harness.Case) {
		c.Fixture("commands/trailing-semicolon.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "non-final command ending in ; does not hang")
		c.EventuallyEqual("both ran", "a,b", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_exit", func(c *harness.Case) {
		c.Fixture("commands/exit.glaze")
		r := c.UpWith(harness.Opts{Timeout: hangTimeout})
		c.Finishes(r, "non-final 'exit' command does not hang")
		c.Match("up warns that it stopped waiting", `stopped waiting`, r.Stderr)
		c.SessionGone("the only pane exits, so tmux ends the session", "exit")
		c.Logf("up exits %d after the session ended", r.Code)
	})

	harness.Run(t, "cmd_command_timeout", func(c *harness.Case) {
		c.Fixture("commands/nonfinal-long-running.glaze")
		r := c.UpWith(harness.Opts{Timeout: hangTimeout}, "--command-timeout", "1s")
		c.OK(r, "up with --command-timeout")
		c.Within(r, "up stops waiting after the timeout", quickUpLimit)
		c.Match("up warns about the timeout", `did not finish in time`, r.Stderr)
	})

	harness.Run(t, "cmd_history_bang", func(c *harness.Case) {
		c.Fixture("commands/history-bang.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "a command with ! does not hang")
		c.EventuallyEqual("! is literal", "wow!x,b", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_tab", func(c *harness.Case) {
		c.Fixture("commands/tab.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "a command with a tab does not hang")
		c.EventuallyEqual("the tab is literal", "a\tb,end", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_leading_dash", func(c *harness.Case) {
		c.Fixture("commands/leading-dash.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "a command that starts with - is not a tmux flag")
		c.EventuallyEqual("both commands ran", "d,e", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_final_keyname", func(c *harness.Case) {
		c.Fixture("commands/final-keyname.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "up")
		c.EventuallyMatch("a final key name runs as a command", `Enter.*not found`, func() string { return c.Tmux("capture-pane", "-p", "-t", "=final-keyname:w") })
	})

	harness.Run(t, "cmd_long_line", func(c *harness.Case) {
		long := strings.Repeat("A", longCommandLength)
		c.Fixture("commands/long-line.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}, "--var", "text="+long), "a 5000-character command does not hang")
		c.EventuallyEqual("the whole line ran", long+",end", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_syntax_error", func(c *harness.Case) {
		c.Fixture("commands/syntax-error.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "a command with a syntax error does not hang")
		c.EventuallyEqual("the commands after it ran", "a,b,end", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_unset_tmux", func(c *harness.Case) {
		c.Fixture("commands/unset-tmux.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "unset TMUX does not hang")
		c.EventuallyEqual("both commands ran", "a,b", fileLines(c, "o"))
	})

	// Each shell reads the commands in its own way, so the same profile runs in each one.
	for _, shell := range []string{"zsh", "fish", "dash"} {
		harness.Run(t, "cmd_shell_"+shell, func(c *harness.Case) {
			path, err := exec.LookPath(shell)
			if err != nil {
				c.T().Skip(shell + " is not installed")
			}

			c.TmuxConf(fmt.Sprintf("set -g default-shell %s\n", path))
			// An empty .zshrc stops the menu that zsh shows to a new user.
			c.Write(filepath.Join(c.Home, ".zshrc"), "")
			c.Mkdir("d")
			c.Fixture("commands/shells.glaze")
			up := c.OK(c.UpWith(harness.Opts{Timeout: shellsTimeout}), fmt.Sprintf("up with %s as the default shell", shell))
			ran := c.EventuallyEqual(fmt.Sprintf("commands ran as written in %s", shell), "wow!x,a\tb,"+c.Path("d")+",end", fileLines(c, "o"))
			if !up || !ran {
				c.Snapshot("shells")
			}

			c.EventuallyEqual(fmt.Sprintf("no glaze buffer is left in %s", shell), "", pasteBuffers(c))
		})
	}

	harness.Run(t, "cmd_special_chars", func(c *harness.Case) {
		c.Fixture("commands/special-chars.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("quotes, ; and $VAR pass through", "a;b|c d|"+c.Home, fileLines(c, "o"))
	})

	harness.Run(t, "cmd_keyname_like", func(c *harness.Case) {
		c.Fixture("commands/keyname-like.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("key-name-like words are literal", "Enter,C-c,Space", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_bare_keyname", func(c *harness.Case) {
		c.Fixture("commands/bare-keyname.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "a command that is exactly a tmux key name (Escape)")
		c.EventuallyEqual("the commands around it ran", "first,third", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_multiline", func(c *harness.Case) {
		c.Fixture("commands/multiline.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "a command containing a newline")
		c.EventuallyEqual("both lines and the next command ran", "one,two,three", fileLines(c, "o"))
	})

	harness.Run(t, "cmd_pane_dir", func(c *harness.Case) {
		c.Mkdir("pd")
		c.Fixture("commands/pane-dir.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("command runs in pane dir", c.Path("pd"), fileLines(c, "o"))
	})

	// The second pane starts after the first pane signals, which is just before its last command.
	harness.Run(t, "cmd_many_panes_serial", func(c *harness.Case) {
		c.Fixture("commands/many-panes.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyMatch("the commands of the first pane start before the second pane", `^a,(a2,b,b2|b,a2,b2|b,b2,a2)$`, fileLines(c, "o"))
	})

	harness.Run(t, "cmd_path_override_env", func(c *harness.Case) {
		c.Fixture("commands/path-override.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: hangTimeout}), "envs.PATH without tmux, then two commands")
	})

	harness.Run(t, "cmd_session", func(c *harness.Case) {
		c.Mkdir("d1", "d2")
		c.Fixture("commands/session.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("session commands ran in order in the active pane", "s1,"+c.Path("d2"), fileLines(c, "o"))
	})

	// Each name makes a new session on the same server, so the names share one case.
	harness.Run(t, "cmd_session_name", func(c *harness.Case) {
		c.Fixture("commands/session-name.glaze")
		for i, name := range []string{"my sess", "semi;colon", "quote'd"} {
			outFile := fmt.Sprintf("o%d", i+1)
			r := c.UpWith(harness.Opts{Timeout: hangTimeout}, "--var", "name="+name, "--var", "out="+outFile)
			c.OK(r, fmt.Sprintf("session commands with session name [%s]", name))
			c.EventuallyEqual(fmt.Sprintf("session commands ran [%s]", name), "a,b", fileLines(c, outFile))
		}
	})

	harness.Run(t, "cmd_many_windows_serial", func(c *harness.Case) {
		c.Fixture("commands/many-windows.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("all commands ran", "1,2,3,4", fileLines(c, "o"))
	})

	// The command text can hold a secret, so glaze prints it only at the debug level.
	harness.Run(t, "cmd_text_at_debug", func(c *harness.Case) {
		c.Fixture("commands/command-text.glaze")
		r := c.Up()
		c.OK(r, "up with a command")
		c.NoMatch("the default log level does not print command text", `hunter3`, r.Output())
		c.Match("the default log level counts the commands", `running pane commands.*count=1`, r.Stderr)
		c.KillServer()
		r = c.Up("--debug")
		c.OK(r, "up --debug with a command")
		c.Match("--debug prints command text", `hunter3`, r.Stderr)
	})
}

// pasteBuffers returns a function that lists the paste buffers of the server. Give it to EventuallyEqual.
func pasteBuffers(c *harness.Case) func() string {
	return func() string { return c.Tmux("list-buffers") }
}
