package tmux

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCommandError(t *testing.T) {
	ce := NewCommandError([]string{"tmux", "ls"}, errors.New("boom"))
	assert.Equal(t, 0, ce.ExitStatus)
	assert.Equal(t, "boom (command: tmux ls)", ce.Error())
}

func TestCommandErrorUnwrap(t *testing.T) {
	cause := errors.New("boom")
	assert.ErrorIs(t, NewCommandError([]string{"tmux", "ls"}, cause), cause)
	assert.ErrorIs(t, NewCommandErrorWithOutput([]string{"tmux", "ls"}, cause, "x"), cause)
}

func TestNewCommandErrorWithOutput(t *testing.T) {
	cewo := NewCommandErrorWithOutput([]string{"tmux", "ls"}, errors.New("boom"), "\nsome output\n")
	assert.Equal(t, "some output", cewo.Output)
	assert.Equal(t, "some output (exit status 0, command: tmux ls)", cewo.Error())
}

func TestCommandErrorWithOutputWithoutOutput(t *testing.T) {
	cewo := NewCommandErrorWithOutput([]string{"tmux", "ls"}, errors.New("exit status 1"), "\n")
	assert.Equal(t, "exit status 1 (command: tmux ls)", cewo.Error())
}

func TestExitStatus(t *testing.T) {
	t.Run("returns zero for a non-exit error", func(t *testing.T) {
		assert.Equal(t, 0, exitStatus(errors.New("plain")))
	})

	t.Run("returns the exit status for an exec.ExitError", func(t *testing.T) {
		err := exec.Command("false").Run()
		var exitErr *exec.ExitError
		assert.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 1, exitStatus(err))
	})
}
