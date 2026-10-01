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
	return fmt.Sprintf("%s (command: %s)", ce.err, strings.Join(ce.args, " "))
}

// Unwrap returns the error from running the command.
func (ce CommandError) Unwrap() error {
	return ce.err
}

// CommandErrorWithOutput extends the CommandError struct with the output of the command.
type CommandErrorWithOutput struct {
	Output string
	CommandError
}

// Error returns what tmux printed, or the error from running the command when tmux printed nothing.
func (cewo CommandErrorWithOutput) Error() string {
	if cewo.Output == "" {
		return cewo.CommandError.Error()
	}

	return fmt.Sprintf("%s (exit status %d, command: %s)", cewo.Output, cewo.ExitStatus, strings.Join(cewo.args, " "))
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
