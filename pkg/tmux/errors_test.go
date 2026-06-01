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
	assert.Equal(t, `error: "boom" status: "0" command: "tmux ls"`, ce.Error())
}

func TestNewCommandErrorWithOutput(t *testing.T) {
	cewo := NewCommandErrorWithOutput([]string{"tmux", "ls"}, errors.New("boom"), "\nsome output\n")
	assert.Equal(t, "some output", cewo.Output)
	assert.Equal(t, `error: "some output" status: "0" command: "tmux ls"`, cewo.Error())
}

func TestReturnExitStatusFromError(t *testing.T) {
	t.Run("returns zero for a non-exit error", func(t *testing.T) {
		assert.Equal(t, 0, returnExitStatusFromError(errors.New("plain")))
	})

	t.Run("returns the exit status for an exec.ExitError", func(t *testing.T) {
		err := exec.Command("false").Run()
		var exitErr *exec.ExitError
		assert.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 1, returnExitStatusFromError(err))
	})
}
