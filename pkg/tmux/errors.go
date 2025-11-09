package tmux

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// CommandError represents an error that occurred while running a command.
type CommandError struct {
	err        error
	args       []string
	ExitStatus int
}

// NewCommandError returns a new command error.
func NewCommandError(args []string, err error) CommandError {
	return CommandError{
		args:       args,
		err:        err,
		ExitStatus: returnExitStatusFromError(err),
	}
}

// Error returns the error message.
func (ce CommandError) Error() string {
	return fmt.Sprintf(`error: "%s" status: "%d" command: "%s"`, ce.err, ce.ExitStatus, strings.Join(ce.args, " "))
}

// CommandErrorWithOutput extends the CommandError struct with the output of the command.
type CommandErrorWithOutput struct {
	Output string
	CommandError
}

// Error returns the error message.
func (cewo CommandErrorWithOutput) Error() string {
	return fmt.Sprintf(`error: "%s" status: "%d" command: "%s"`, cewo.Output, cewo.ExitStatus, strings.Join(cewo.args, " "))
}

// NewCommandError returns a new command error.
func NewCommandErrorWithOutput(args []string, err error, output string) CommandErrorWithOutput {
	return CommandErrorWithOutput{
		CommandError: CommandError{
			args:       args,
			err:        err,
			ExitStatus: returnExitStatusFromError(err),
		},
		Output: strings.Trim(output, "\n"),
	}
}

// returnExitStatusFromError derives the exit status code from the given error.
func returnExitStatusFromError(err error) int {
	exitStatus := 0
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			exitStatus = status.ExitStatus()
		}
	}

	return exitStatus
}
