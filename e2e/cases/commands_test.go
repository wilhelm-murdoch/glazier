package cases

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// commandsTimeout is the deadline of an up whose commands can hang glaze.
const commandsTimeout = 10 * time.Second

func TestCommands(t *testing.T) {
	// out returns the lines that the panes wrote to o, joined with commas.
	out := func(c *harness.Case) func() string { return func() string { return c.Lines("o") } }

	harness.Run(t, "cmd_serial", func(c *harness.Case) {
		c.Fixture("commands/serial.glaze")
		r := c.Up()
		c.OK(r, "up")
		c.AtLeast(r, "up waits for non-final commands", 1900*time.Millisecond)
		c.EventuallyEqual("commands ran in order", "a,b,c", out(c))
		c.EventuallyEqual("no glaze buffer is left", "", func() string { return c.Tmux("list-buffers") })
	})

	harness.Run(t, "cmd_last_long_running", func(c *harness.Case) {
		c.Fixture("commands/last-long-running.glaze")
		r := c.UpWith(harness.Opts{Timeout: 15 * time.Second})
		c.OK(r, "up with long-running final command")
		c.Within(r, "final command is fire-and-forget", 5*time.Second)
	})

	harness.Run(t, "cmd_single_long_running", func(c *harness.Case) {
		c.Fixture("commands/single-long-running.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: 15 * time.Second}), "up with single long-running command")
	})

	// glaze has no default limit for a command, so it waits until the deadline of the case.
	harness.Run(t, "cmd_nonfinal_long_running", func(c *harness.Case) {
		c.Fixture("commands/nonfinal-long-running.glaze")
		r := c.UpWith(harness.Opts{Timeout: 3 * time.Second})
		c.True("non-final long-running command blocks up (by design)", r.TimedOut, "up ended with exit %d after %s", r.Code, r.Duration)
	})

	harness.Run(t, "cmd_failing", func(c *harness.Case) {
		c.Fixture("commands/failing.glaze")
		c.OK(c.Up(), "up with a failing command")
		c.EventuallyEqual("later commands still run", "after", out(c))
	})

	harness.Run(t, "cmd_exit_then_split", func(c *harness.Case) {
		c.Fixture("commands/exit-then-split.glaze")
		c.OK(c.Up(), "a pane whose command exits does not break the next split")
		c.EventuallyEqual("the other panes keep their order", "shell,shell2", func() string { return c.PaneTitles("=ex:w") })
	})

	harness.Run(t, "cmd_trailing_comment", func(c *harness.Case) {
		c.Fixture("commands/trailing-comment.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "non-final command with trailing # comment does not hang")
		c.EventuallyEqual("both commands ran", "a,b", out(c))
	})

	harness.Run(t, "cmd_trailing_ampersand", func(c *harness.Case) {
		c.Fixture("commands/trailing-ampersand.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "non-final backgrounded command (&) does not hang")
		c.EventuallyEqual("command after & ran", "b", out(c))
	})

	harness.Run(t, "cmd_trailing_semicolon", func(c *harness.Case) {
		c.Fixture("commands/trailing-semicolon.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "non-final command ending in ; does not hang")
		c.EventuallyEqual("both ran", "a,b", out(c))
	})

	harness.Run(t, "cmd_exit", func(c *harness.Case) {
		c.Fixture("commands/exit.glaze")
		r := c.UpWith(harness.Opts{Timeout: 8 * time.Second})
		c.Finishes(r, "non-final 'exit' command does not hang")
		c.Match("up warns that it stopped waiting", `stopped waiting`, r.Stderr)
		c.SessionGone("the only pane exits, so tmux ends the session", "ce")
		c.Logf("up exits %d after the session ended", r.Code)
	})

	harness.Run(t, "cmd_command_timeout", func(c *harness.Case) {
		c.Fixture("commands/nonfinal-long-running.glaze")
		r := c.UpWith(harness.Opts{Timeout: commandsTimeout}, "--command-timeout", "1s")
		c.OK(r, "up with --command-timeout")
		c.Within(r, "up stops waiting after the timeout", 5*time.Second)
		c.Match("up warns about the timeout", `did not finish in time`, r.Stderr)
	})

	harness.Run(t, "cmd_history_bang", func(c *harness.Case) {
		c.Fixture("commands/history-bang.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "a command with ! does not hang")
		c.EventuallyEqual("! is literal", "wow!x,b", out(c))
	})

	harness.Run(t, "cmd_tab", func(c *harness.Case) {
		c.Fixture("commands/tab.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "a command with a tab does not hang")
		c.EventuallyEqual("the tab is literal", "a\tb,end", out(c))
	})

	harness.Run(t, "cmd_leading_dash", func(c *harness.Case) {
		c.Fixture("commands/leading-dash.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "a command that starts with - is not a tmux flag")
		c.EventuallyEqual("both commands ran", "d,e", out(c))
	})

	harness.Run(t, "cmd_final_keyname", func(c *harness.Case) {
		c.Fixture("commands/final-keyname.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "up")
		c.EventuallyMatch("a final key name runs as a command", `Enter.*not found`, func() string { return c.Tmux("capture-pane", "-p", "-t", "=fk:w") })
	})

	harness.Run(t, "cmd_long_line", func(c *harness.Case) {
		long := strings.Repeat("A", 5000)
		c.Write(".glaze", fmt.Sprintf("session {\n  name = \"ll\"\n  window {\n    name = \"w\"\n    pane {\n      name     = \"p\"\n      commands = %s\n    }\n  }\n}\n",
			harness.List("echo "+long+" > o", "echo end >> o")))
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "a 5000-character command does not hang")
		c.EventuallyEqual("the whole line ran", long+",end", out(c))
	})

	harness.Run(t, "cmd_syntax_error", func(c *harness.Case) {
		c.Fixture("commands/syntax-error.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "a command with a syntax error does not hang")
		c.EventuallyEqual("the commands after it ran", "a,b,end", out(c))
	})

	harness.Run(t, "cmd_unset_tmux", func(c *harness.Case) {
		c.Fixture("commands/unset-tmux.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "unset TMUX does not hang")
		c.EventuallyEqual("both commands ran", "a,b", out(c))
	})

	// Each shell reads the commands in its own way, so the same profile runs in each one.
	for _, shell := range []string{"zsh", "fish", "dash"} {
		harness.Run(t, "cmd_shell_"+shell, func(c *harness.Case) {
			path, err := exec.LookPath(shell)
			if err != nil {
				c.Logf("default shell %s: not installed", shell)
				return
			}
			c.TmuxConf(fmt.Sprintf("set -g default-shell %s\n", path))
			c.Write("home/.zshrc", "")
			c.Mkdir("d")
			c.Fixture("commands/shells.glaze")
			c.OK(c.UpWith(harness.Opts{Timeout: 15 * time.Second}), "up with "+shell+" as the default shell")
			c.EventuallyEqual("commands ran as written in "+shell, "wow!x,a\tb,"+c.Path("d")+",end", out(c))
			c.EventuallyEqual("no glaze buffer is left in "+shell, "", func() string { return c.Tmux("list-buffers") })
		})
	}

	harness.Run(t, "cmd_special_chars", func(c *harness.Case) {
		c.Fixture("commands/special-chars.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("quotes, ; and $VAR pass through", "a;b|c d|"+c.Home, out(c))
	})

	harness.Run(t, "cmd_keyname_like", func(c *harness.Case) {
		c.Fixture("commands/keyname-like.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("key-name-like words are literal", "Enter,C-c,Space", out(c))
	})

	harness.Run(t, "cmd_bare_keyname", func(c *harness.Case) {
		c.Fixture("commands/bare-keyname.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: 8 * time.Second}), "a command that is exactly a tmux key name (Escape)")
		c.EventuallyEqual("the commands around it ran", "first,third", out(c))
	})

	harness.Run(t, "cmd_multiline", func(c *harness.Case) {
		c.Fixture("commands/multiline.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}), "a command containing a newline")
		c.EventuallyEqual("both lines and the next command ran", "one,two,three", out(c))
	})

	harness.Run(t, "cmd_pane_dir", func(c *harness.Case) {
		c.Mkdir("pd")
		c.Fixture("commands/pane-dir.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("command runs in pane dir", c.Path("pd"), out(c))
	})

	// The second pane starts after the first pane signals, which is just before its last command.
	harness.Run(t, "cmd_many_panes_serial", func(c *harness.Case) {
		c.Fixture("commands/many-panes.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyMatch("the commands of the first pane start before the second pane", `^a,(a2,b,b2|b,a2,b2|b,b2,a2)$`, out(c))
		c.Logf("cross-pane command order: %s", c.Lines("o"))
	})

	harness.Run(t, "cmd_path_override_env", func(c *harness.Case) {
		c.Fixture("commands/path-override.glaze")
		c.OK(c.UpWith(harness.Opts{Timeout: 8 * time.Second}), "envs.PATH without tmux, then two commands")
	})

	harness.Run(t, "session_commands", func(c *harness.Case) {
		c.Mkdir("d1", "d2")
		c.Fixture("commands/session.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("session commands ran in order in the active pane", "s1,"+c.Path("d2"), out(c))
	})

	// Each name makes a new session on the same server, so the names share one case.
	harness.Run(t, "session_commands_name", func(c *harness.Case) {
		c.Fixture("commands/session-name.glaze")
		for i, name := range []string{"my sess", "semi;colon", "quote'd"} {
			o := fmt.Sprintf("o%d", i+1)
			c.OK(c.UpWith(harness.Opts{Timeout: commandsTimeout}, "--var", "name="+name, "--var", "out="+o), fmt.Sprintf("session commands with session name [%s]", name))
			c.EventuallyEqual(fmt.Sprintf("session commands ran [%s]", name), "a,b", func() string { return c.Lines(o) })
		}
	})

	harness.Run(t, "pane_commands_many_windows_serial", func(c *harness.Case) {
		c.Fixture("commands/many-windows.glaze")
		c.OK(c.Up(), "up")
		c.EventuallyMatch("all commands ran", `1.*2.*3`, out(c))
	})
}
