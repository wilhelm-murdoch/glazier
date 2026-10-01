package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// Commander is an interface that represents what kind of actions a Command, and
// other implemenations, can perform.
type Commander interface {
	fmt.Stringer
	Exec() error
	ExecWithOutput() (string, error)
	ExecWithInput(input string) error
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

// OverrideCommandFactory replaces the factory used to construct tmux commands
// and returns a function that restores the previous factory. It exists so that
// packages which drive a Client (such as the CLI actions) can substitute a fake
// Commander in tests without spawning a real tmux process. Production code must
// not call this.
func OverrideCommandFactory(factory func(client Client, args ...string) Commander) func() {
	previous := newCommand
	newCommand = factory
	return func() {
		newCommand = previous
	}
}

// Command represents a command to run within a tmux session.
type Command struct {
	cmd  *exec.Cmd
	args []string
}

// NewCommand returns a new command with the given arguments.
func NewCommand(client Client, args ...string) *Command {
	args = escapeSeparators(args)

	// Glaze reads output as UTF-8, so stop tmux printing non-ASCII as "_" under a non-UTF-8 locale.
	// An attached client is the user's terminal, so its locale decides.
	if subcommandOf(args) != "attach" {
		args = append([]string{"-u"}, args...)
	}

	if client.socketName != "" {
		args = append([]string{"-L", client.socketName}, args...)
	} else if client.socketPath != "" {
		args = append([]string{"-S", client.socketPath}, args...)
	}

	args = append([]string{client.tmuxPath}, args...)

	return &Command{
		args: args,
		// Spawning tmux with caller-supplied arguments is this package's
		// entire purpose; args[0] is the resolved tmux binary path.
		cmd: exec.Command(args[0], args[1:]...), //nolint:gosec // G204
	}
}

// subcommandOf returns the tmux command in args, skipping any socket flags.
func subcommandOf(args []string) string {
	if i := subcommandIndex(args); i < len(args) {
		return args[i]
	}

	return ""
}

// subcommandIndex returns the position of the tmux command in args, skipping any socket flags.
func subcommandIndex(args []string) int {
	i := 0
	for i < len(args) && (args[i] == "-L" || args[i] == "-S") {
		i += 2
	}

	return min(i, len(args))
}

// escapeSeparators puts a backslash before a trailing ; in each argument of the tmux command.
// tmux reads an unescaped trailing ; as the end of the command, even in a name or a path.
func escapeSeparators(args []string) []string {
	escaped := slices.Clone(args)
	for i := subcommandIndex(args); i < len(escaped); i++ {
		if strings.HasSuffix(escaped[i], ";") {
			escaped[i] = strings.TrimSuffix(escaped[i], ";") + `\;`
		}
	}

	return escaped
}

// String returns the full command with arguments as a string.
func (c Command) String() string {
	return strings.Join(c.args, " ")
}

// Exec executes the command and puts the output of tmux into the error when it fails.
// Only attach keeps the terminal, because an attached client needs it.
func (c *Command) Exec() error {
	if subcommandOf(c.args[1:]) != "attach" {
		if output, err := c.cmd.CombinedOutput(); err != nil {
			return NewCommandErrorWithOutput(c.args, err, string(output))
		}

		return nil
	}

	c.cmd.Stdin = os.Stdin
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr

	if err := c.cmd.Run(); err != nil {
		return NewCommandError(c.args, err)
	}

	return nil
}

// ExecWithOutput executes the command and returns the output as a string.
func (c Command) ExecWithOutput() (string, error) {
	output, err := c.cmd.CombinedOutput()
	if err != nil {
		return "", NewCommandErrorWithOutput(c.args, err, string(output))
	}

	return strings.TrimSuffix(string(output), "\n"), nil
}

// ExecWithInput executes the command with input on its stdin, so the input never appears in the argument list.
func (c Command) ExecWithInput(input string) error {
	c.cmd.Stdin = strings.NewReader(input)

	output, err := c.cmd.CombinedOutput()
	if err != nil {
		return NewCommandErrorWithOutput(c.args, err, string(output))
	}

	return nil
}
