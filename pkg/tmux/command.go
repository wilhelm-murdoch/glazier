package tmux

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"
)

var (
	// Ensure Command properly implements the Commander interface.
	_ Commander = (*Command)(nil)

	// Assign NewCommand to newCommand so we can test its functionality via
	// dependency injection.
	newCommand = func(client Client, args ...string) Commander {
		return NewCommand(client, args...)
	}

	// cancelGrace is how long a cancelled tmux command gets to exit after SIGTERM before it gets SIGKILL.
	cancelGrace = 5 * time.Second
)

// Commander is an interface that represents what kind of actions a Command, and
// other implemenations, can perform.
type Commander interface {
	fmt.Stringer
	Exec() error
	ExecWithOutput() (string, error)
	ExecWithInput(input string) error
}

// OverrideCommandFactory replaces the factory for tmux commands and returns a function that restores it.
// It lets tests in other packages fake tmux. Production code must not call it.
func OverrideCommandFactory(factory func(client Client, args ...string) Commander) func() {
	previous := newCommand
	newCommand = factory
	return func() {
		newCommand = previous
	}
}

// Command is one tmux command, ready to run.
type Command struct {
	cmd    *exec.Cmd
	args   []string
	logger *slog.Logger
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

	// Spawning tmux with caller-supplied arguments is this package's
	// entire purpose; args[0] is the resolved tmux binary path.
	cmd := exec.CommandContext(client.context(), args[0], args[1:]...) //nolint:gosec // G204

	// A tmux client restores the terminal on SIGTERM, but not on the SIGKILL that a cancelled context sends by default.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = cancelGrace

	return &Command{args: args, cmd: cmd, logger: client.logger}
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

// debug logs the command before it runs, so --debug shows every command that glaze sends.
func (c Command) debug() {
	if c.logger != nil {
		c.logger.Debug(c.String())
	}
}

// Exec executes the command and puts the output of tmux into the error when it fails.
// Only attach keeps the terminal, because an attached client needs it.
func (c *Command) Exec() error {
	c.debug()

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
	c.debug()

	output, err := c.cmd.CombinedOutput()
	if err != nil {
		return "", NewCommandErrorWithOutput(c.args, err, string(output))
	}

	return strings.TrimSuffix(string(output), "\n"), nil
}

// ExecWithInput executes the command with input on its stdin, so the input never appears in the argument list.
func (c Command) ExecWithInput(input string) error {
	c.debug()
	c.cmd.Stdin = strings.NewReader(input)

	output, err := c.cmd.CombinedOutput()
	if err != nil {
		return NewCommandErrorWithOutput(c.args, err, string(output))
	}

	return nil
}
