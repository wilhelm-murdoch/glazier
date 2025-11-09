package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Command represents a command to run within a tmux session.
type Command struct {
	cmd  *exec.Cmd
	args []string
}

// NewCommand returns a new command with the given arguments.
func NewCommand(client Client, args ...string) (Command, error) {
	ok, tmux := IsInstalled()
	if !ok {
		return Command{}, fmt.Errorf("tmux is not installed")
	}

	if client.socketName != "" {
		args = append([]string{"-L", client.socketName}, args...)
	} else if client.socketPath != "" {
		args = append([]string{"-S", client.socketPath}, args...)
	}

	args = append([]string{tmux}, args...)

	if client.debug {
		fmt.Println(args)
	}

	return Command{
		args: args,
		cmd:  exec.Command(args[0], args[1:]...),
	}, nil
}

// String returns the full command with arguments as a string.
func (c Command) String() string {
	return strings.Join(c.args, " ")
}

// Exec executes the command and returns an error if one occurred. It will pipe
// any output to os.Stdin, os.Stdout and os.Stderr.
func (c Command) Exec() error {
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
