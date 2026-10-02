package tmux

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCommandArgs(t *testing.T) {
	t.Run("prepends the socket name when set", func(t *testing.T) {
		client := Client{tmuxPath: "tmux", socketName: "sock"}
		cmd := NewCommand(client, "ls", "-F", "x")
		assert.Equal(t, []string{"tmux", "-L", "sock", "-u", "ls", "-F", "x"}, cmd.args)
	})

	t.Run("prepends the socket path when set", func(t *testing.T) {
		client := Client{tmuxPath: "tmux", socketPath: "/tmp/tmux.sock"}
		cmd := NewCommand(client, "ls")
		assert.Equal(t, []string{"tmux", "-S", "/tmp/tmux.sock", "-u", "ls"}, cmd.args)
	})

	t.Run("prefers the socket name over the socket path", func(t *testing.T) {
		client := Client{tmuxPath: "tmux", socketName: "sock", socketPath: "/tmp/tmux.sock"}
		cmd := NewCommand(client, "ls")
		assert.Equal(t, []string{"tmux", "-L", "sock", "-u", "ls"}, cmd.args)
	})

	t.Run("uses only the tmux path when no socket is set", func(t *testing.T) {
		client := Client{tmuxPath: "tmux"}
		cmd := NewCommand(client, "info")
		assert.Equal(t, []string{"tmux", "-u", "info"}, cmd.args)
	})

	t.Run("leaves attach to the user's locale", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "tmux"}, "-L", "sock", "attach", "-t", "$1")
		assert.Equal(t, []string{"tmux", "-L", "sock", "attach", "-t", "$1"}, cmd.args)
	})

	t.Run("escapes a trailing ; in each argument", func(t *testing.T) {
		args := []string{"splitw", "-c", "/srv/d;", "-t", "%1", ";", `p\;`, "a;b"}
		cmd := NewCommand(Client{tmuxPath: "tmux"}, args...)
		assert.Equal(t, []string{"tmux", "-u", "splitw", "-c", `/srv/d\;`, "-t", "%1", `\;`, `p\\;`, "a;b"}, cmd.args)
		assert.Equal(t, "/srv/d;", args[2], "the caller's arguments must not change")
	})

	t.Run("does not escape the socket flags", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "tmux"}, "-L", "sock;", "attach", "-t", "s;")
		assert.Equal(t, []string{"tmux", "-L", "sock;", "attach", "-t", `s\;`}, cmd.args)
	})
}

func TestCommandString(t *testing.T) {
	cmd := NewCommand(Client{tmuxPath: "tmux", socketName: "sock"}, "ls", "-F", "x")
	assert.Equal(t, "tmux -L sock -u ls -F x", cmd.String())
}

func TestCommandExec(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "true"})
		assert.NoError(t, cmd.Exec())
	})

	t.Run("puts the output of a failed command into the error", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "sh"}, "-c", "echo can\\'t find session: x >&2; exit 3")
		err := cmd.Exec()

		var withOutput CommandErrorWithOutput
		assert.ErrorAs(t, err, &withOutput)
		assert.Equal(t, "can't find session: x", withOutput.Output)
		assert.Equal(t, 3, withOutput.ExitStatus)
	})

	t.Run("does not print the output of a command that succeeds", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "sh"}, "-c", "echo noise")
		assert.NoError(t, cmd.Exec())
	})
}

func TestCommandLogsAtDebugLevel(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	assert.NoError(t, NewCommand(Client{tmuxPath: "true", logger: logger}, "ls").Exec())
	assert.Contains(t, logs.String(), "level=DEBUG msg=\"true -u ls\"")

	_, err := NewCommand(Client{tmuxPath: "true", logger: logger}, "lsw").ExecWithOutput()
	assert.NoError(t, err)
	assert.Contains(t, logs.String(), "true -u lsw")
}

func TestRedactSecrets(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"an env value", []string{"setenv", "-t", "$1", "TOKEN", "hunter2"}, []string{"setenv", "-t", "$1", "TOKEN", redactedValue}},
		{"the long command name", []string{"set-environment", "-g", "TOKEN", "hunter2"}, []string{"set-environment", "-g", "TOKEN", redactedValue}},
		{"a value that looks like a flag", []string{"setenv", "-t", "$1", "OPT", "-t"}, []string{"setenv", "-t", "$1", "OPT", redactedValue}},
		{"after the socket flags", []string{"-L", "s", "setenv", "K", "v"}, []string{"-L", "s", "setenv", "K", redactedValue}},
		{"an unset has no value", []string{"setenv", "-u", "-t", "$1", "TOKEN"}, []string{"setenv", "-u", "-t", "$1", "TOKEN"}},
		{"another command", []string{"set-option", "-t", "$1", "status", "off"}, []string{"set-option", "-t", "$1", "status", "off"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, redactSecrets(tc.args))
		})
	}
}

func TestCommandRedactsEnvValues(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cmd := NewCommand(Client{tmuxPath: "false", logger: logger}, "setenv", "-t", "$1", "TOKEN", "hunter2")
	assert.Equal(t, "hunter2", cmd.args[len(cmd.args)-1], "tmux must still get the value")

	err := cmd.Exec()
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2")
	assert.Contains(t, err.Error(), "TOKEN "+redactedValue)
	assert.NotContains(t, logs.String(), "hunter2")
	assert.Contains(t, cmd.String(), "TOKEN "+redactedValue)
}

func TestCommandContext(t *testing.T) {
	t.Run("a cancelled context stops the command with SIGTERM", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := Client{tmuxPath: "sh"}.WithContext(ctx)
		cmd := NewCommand(client, "-c", "trap 'echo got TERM; kill $!; exit 7' TERM; sleep 5 & wait")

		time.AfterFunc(200*time.Millisecond, cancel)
		err := cmd.Exec()

		var withOutput CommandErrorWithOutput
		assert.ErrorAs(t, err, &withOutput)
		assert.Equal(t, "got TERM", withOutput.Output)
		assert.Equal(t, 7, withOutput.ExitStatus)
	})

	t.Run("WithoutCancel runs commands after a cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		client := Client{tmuxPath: "true"}.WithContext(ctx)
		assert.Error(t, NewCommand(client).Exec())
		assert.NoError(t, NewCommand(client.WithoutCancel()).Exec())
	})
}

func TestCommandExecWithOutput(t *testing.T) {
	t.Run("returns trimmed output on success", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "echo"}, "hello world")
		out, err := cmd.ExecWithOutput()
		assert.NoError(t, err)
		assert.Equal(t, "-u hello world", out)
	})

	t.Run("returns a wrapped error on failure", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "false"})
		out, err := cmd.ExecWithOutput()
		assert.Error(t, err)
		assert.Equal(t, "", out)
		assert.IsType(t, CommandErrorWithOutput{}, err)
	})
}
