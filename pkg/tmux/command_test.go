package tmux

import (
	"testing"

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

	t.Run("returns a wrapped error on failure", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "false"})
		err := cmd.Exec()
		assert.Error(t, err)
		assert.IsType(t, CommandError{}, err)
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

func TestCommandExecWithStatus(t *testing.T) {
	t.Run("returns zero on success", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "true"})
		assert.Equal(t, 0, cmd.ExecWithStatus())
	})

	t.Run("returns non-zero on failure", func(t *testing.T) {
		cmd := NewCommand(Client{tmuxPath: "false"})
		assert.Equal(t, 1, cmd.ExecWithStatus())
	})
}
