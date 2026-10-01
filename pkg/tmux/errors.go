package tmux

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	// ErrUnreachable means that glaze cannot run tmux or cannot connect to the tmux server.
	ErrUnreachable = errors.New("tmux is unreachable")

	// ErrOtherServer means that glaze runs inside a different tmux server, so attaching would nest a client in a pane.
	ErrOtherServer = errors.New("glaze runs inside a different tmux server")

	// ErrDuplicateSession means that another client created a session with the same name first.
	ErrDuplicateSession = errors.New("a session with this name already exists")

	// ErrPaneExited means that the shell of a pane exited before it signalled that its commands finished.
	ErrPaneExited = errors.New("the pane's shell exited before its commands finished")

	// ErrCommandTimeout means that the commands of a pane did not finish within --command-timeout.
	ErrCommandTimeout = errors.New("the pane's commands did not finish in time")

	// ErrTrailingEscape means that a line of tmux output ends in a lone backslash, so it is truncated or corrupt.
	ErrTrailingEscape = errors.New("tmux: line ends in an unpaired escape character")

	// ErrUnexpectedPartCount means that a line of tmux output has a different number of fields than its format.
	ErrUnexpectedPartCount = errors.New("tmux: unexpected number of parts in line")

	// ErrInvalidDerivedId means that the first field of a line of tmux output is not a valid id.
	ErrInvalidDerivedId = errors.New("tmux: line has an invalid id")

	// ErrIdPrefixNotFound means that an id field does not start with the expected $, @ or %.
	ErrIdPrefixNotFound = errors.New("tmux: id prefix not found")
)

// CommandError is a tmux command that could not run or that failed.
type CommandError struct {
	err        error
	args       []string
	ExitStatus int
}

// NewCommandError returns the error for the command args that failed with err.
func NewCommandError(args []string, err error) CommandError {
	return CommandError{args: args, err: err, ExitStatus: exitStatus(err)}
}

func (ce CommandError) Error() string {
	return fmt.Sprintf("%s (command: %s)", ce.err, strings.Join(ce.args, " "))
}

// Unwrap returns the error from running the command.
func (ce CommandError) Unwrap() error {
	return ce.err
}

// CommandErrorWithOutput is a failed tmux command together with what tmux printed.
type CommandErrorWithOutput struct {
	Output string
	CommandError
}

// NewCommandErrorWithOutput returns the error for the command args that failed with err and printed output.
func NewCommandErrorWithOutput(args []string, err error, output string) CommandErrorWithOutput {
	return CommandErrorWithOutput{CommandError: NewCommandError(args, err), Output: strings.Trim(output, "\n")}
}

// Error returns what tmux printed, or the error from running the command when tmux printed nothing.
func (cewo CommandErrorWithOutput) Error() string {
	if cewo.Output == "" {
		return cewo.CommandError.Error()
	}

	return fmt.Sprintf("%s (exit status %d, command: %s)", cewo.Output, cewo.ExitStatus, strings.Join(cewo.args, " "))
}

// exitStatus returns the exit status of a command that exited, or 0 for any other error.
func exitStatus(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return 0
}

// tmuxSaid reports whether err is a failed tmux command whose output contains one of messages.
func tmuxSaid(err error, messages ...string) bool {
	var withOutput CommandErrorWithOutput
	if !errors.As(err, &withOutput) {
		return false
	}

	for _, message := range messages {
		if strings.Contains(withOutput.Output, message) {
			return true
		}
	}

	return false
}

// lookupFailure returns nil when err only says that the server or the session does not exist, and ErrUnreachable otherwise.
// A missing socket gives "No such file or directory", and a server that stops while another glaze starts it gives "server exited unexpectedly".
func lookupFailure(err error) error {
	if tmuxSaid(err, "can't find session", "no server running on", "(No such file or directory)", "server exited unexpectedly") {
		return nil
	}

	return fmt.Errorf("%w: %w", ErrUnreachable, err)
}
