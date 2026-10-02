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
	cmd  *exec.Cmd
	args []string

	// shown is args for logs and errors, with secret values redacted.
	shown  []string
	logger *slog.Logger
}

// NewCommand returns a new command with the given arguments.
func NewCommand(client Client, args ...string) *Command {
	args = escapeSeparators(args)
	prefix := []string{client.tmuxPath}

	if client.socketName != "" {
		prefix = append(prefix, "-L", client.socketName)
	} else if client.socketPath != "" {
		prefix = append(prefix, "-S", client.socketPath)
	}

	// Glaze reads output as UTF-8, so stop tmux printing non-ASCII as "_" under a non-UTF-8 locale.
	// An attached client is the user's terminal, so its locale decides.
	if subcommandOf(args) != "attach" {
		prefix = append(prefix, "-u")
	}

	shown := slices.Concat(prefix, redactSecrets(args))
	args = slices.Concat(prefix, args)

	// Spawning tmux with caller-supplied arguments is this package's
	// entire purpose; args[0] is the resolved tmux binary path.
	cmd := exec.CommandContext(client.context(), args[0], args[1:]...) //nolint:gosec // G204

	// A tmux client restores the terminal on SIGTERM, but not on the SIGKILL that a cancelled context sends by default.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = cancelGrace

	return &Command{args: args, shown: shown, cmd: cmd, logger: client.logger}
}

// redacted replaces a secret value in logs and errors.
const redacted = "<redacted>"

// redactSecrets returns a copy of args with the value of `setenv NAME VALUE` replaced, because an env value is often a secret.
func redactSecrets(args []string) []string {
	shown := slices.Clone(args)
	if sub := subcommandOf(args); sub != "setenv" && sub != "set-environment" {
		return shown
	}

	// The operands are NAME and VALUE. Flags and the target of -t come before them, so a VALUE such as "-t" is still an operand.
	var operands []int
	for i := subcommandIndex(args) + 1; i < len(args); i++ {
		switch {
		case len(operands) > 0 || !strings.HasPrefix(args[i], "-"):
			operands = append(operands, i)
		case args[i] == "-t":
			i++
		}
	}

	if len(operands) == 2 {
		shown[operands[1]] = redacted
	}

	return shown
}

// cancelGrace is how long a cancelled tmux command gets to exit after SIGTERM before it gets SIGKILL.
var cancelGrace = 5 * time.Second

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

// String returns the full command with arguments as a string, with secret values redacted.
func (c Command) String() string {
	return strings.Join(c.shown, " ")
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
			return NewCommandErrorWithOutput(c.shown, err, string(output))
		}

		return nil
	}

	c.cmd.Stdin = os.Stdin
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr

	if err := c.cmd.Run(); err != nil {
		return NewCommandError(c.shown, err)
	}

	return nil
}

// ExecWithOutput executes the command and returns the output as a string.
func (c Command) ExecWithOutput() (string, error) {
	c.debug()

	output, err := c.cmd.CombinedOutput()
	if err != nil {
		return "", NewCommandErrorWithOutput(c.shown, err, string(output))
	}

	return strings.TrimSuffix(string(output), "\n"), nil
}

// ExecWithInput executes the command with input on its stdin, so the input never appears in the argument list.
func (c Command) ExecWithInput(input string) error {
	c.debug()
	c.cmd.Stdin = strings.NewReader(input)

	output, err := c.cmd.CombinedOutput()
	if err != nil {
		return NewCommandErrorWithOutput(c.shown, err, string(output))
	}

	return nil
}
