package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var defaultTmuxExecutablePath = "tmux"

// Commander is an interface that represents what kind of actions a Command, and
// other implemenations, can perform.
type Commander interface {
	fmt.Stringer
	Exec() error
	ExecWithOutput() (string, error)
	ExecWithStatus() int
}

var (
	// Ensure Command properly implements the Commander interface.
	_ Commander = (*Command)(nil)

	// Assign NewCommand to newCommand so we can test its functionality via
	// dependency injection.
	newCommand = func(client Client, args ...string) Commander {
		return NewCommand(client, args...)
	}
)

// Command represents a command to run within a tmux session.
type Command struct {
	cmd  *exec.Cmd
	args []string
}

// NewCommand returns a new command with the given arguments.
func NewCommand(client Client, args ...string) *Command {
	if client.socketName != "" {
		args = append([]string{"-L", client.socketName}, args...)
	} else if client.socketPath != "" {
		args = append([]string{"-S", client.socketPath}, args...)
	}

	args = append([]string{client.tmuxPath}, args...)

	return &Command{
		args: args,
		cmd:  exec.Command(args[0], args[1:]...),
	}
}

// String returns the full command with arguments as a string.
func (c Command) String() string {
	return strings.Join(c.args, " ")
}

// Exec executes the command and returns an error if one occurred. It will pipe
// any output to os.Stdin, os.Stdout and os.Stderr.
func (c *Command) Exec() error {
	c.cmd.Stdin = os.Stdin
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr

	if err := c.cmd.Run(); err != nil {
		return NewCommandError(c.args, err)
	}

	return nil
}

// ExecWithStatus executes the command and attempts to return its exit status.
func (c Command) ExecWithStatus() int {
	err := c.cmd.Run()
	if err != nil && !errors.Is(err, CommandError{}) {
		return 1
	}

	return returnExitStatusFromError(err)
}

// ExecWithOutput executes the command and returns the output as a string.
func (c Command) ExecWithOutput() (string, error) {
	output, err := c.cmd.CombinedOutput()
	if err != nil {
		return "", NewCommandErrorWithOutput(c.args, err, string(output))
	}

	return strings.TrimSuffix(string(output), "\n"), nil
}
