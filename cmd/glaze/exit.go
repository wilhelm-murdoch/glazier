package main

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// The exit codes of glaze. The README and packaging/glaze.1 (EXIT STATUS) list them, so a change here needs a change in both.
const (
	exitOK             = 0
	exitFailure        = 1
	exitUsage          = 2
	exitInvalidProfile = 3
	exitUnreachable    = 4

	// A run that a signal stops exits with 128 plus the signal number, like a shell: 130 for SIGINT and 143 for SIGTERM.
	exitSignalBase = 128
)

// errUsage marks an error in the command line itself, for example an unknown flag.
var errUsage = errors.New("usage error")

// signalError is the cause of a run that SIGINT or SIGTERM stopped.
type signalError struct {
	signal syscall.Signal
}

func (e signalError) Error() string {
	name := "SIGTERM"
	if e.signal == syscall.SIGINT {
		name = "SIGINT"
	}

	return "glaze stopped on " + name
}

// exitCode returns the exit code for the error that a command returned.
func exitCode(err error) int {
	var stopped signalError

	switch {
	case err == nil:
		return exitOK
	case errors.As(err, &stopped):
		return exitSignalBase + int(stopped.signal)
	case errors.Is(err, errUsage), errors.Is(err, files.ErrInvalidProfileName):
		return exitUsage
	case errors.Is(err, diagnostics.ErrHasDiagnostics), errors.Is(err, files.ErrProfileNotFound), errors.Is(err, files.ErrRegistryUnusable):
		return exitInvalidProfile
	case errors.Is(err, tmux.ErrUnreachable), errors.Is(err, tmux.ErrOldTmux):
		return exitUnreachable
	default:
		return exitFailure
	}
}

// configureCompletion shows the completion command in the help with the wording of the man page.
// It makes an unknown shell a usage error, like an unknown command.
func configureCompletion(cmd *cli.Command) {
	cmd.Hidden = false
	cmd.Usage = "print a completion script for bash, zsh, fish or pwsh"
	cmd.Description = "eval \"$(glaze completion bash)\"        # add to ~/.bashrc\n" +
		"source <(glaze completion zsh)         # add to ~/.zshrc, after compinit\n" +
		"mkdir -p ~/.config/fish/completions && glaze completion fish > ~/.config/fish/completions/glaze.fish"
	cmd.Action = func(_ context.Context, cmd *cli.Command) error {
		if cmd.Args().Present() {
			return fmt.Errorf("%w: unknown shell `%s` (see `glaze completion --help`)", errUsage, cmd.Args().First())
		}

		return cli.ShowSubcommandHelp(cmd)
	}
}

// rootAction shows the help, or returns a usage error for an unknown command.
// The CLI library reports an unknown command with an ExitCoder, which a failed tmux command also unwraps to.
func rootAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Present() {
		word := cmd.Args().First()
		return fmt.Errorf("%w: unknown command `%s`%s (see `glaze --help`)", errUsage, word, profileHint(word))
	}

	return cli.ShowRootCommandHelp(cmd)
}

// profileHint says how to apply a profile when an unknown command names one; any problem on the way gives no hint.
func profileHint(word string) string {
	path, err := files.RegistryProfilePath(word)
	if err != nil || !files.FileExists(path) {
		return ""
	}

	return fmt.Sprintf("; to apply the profile `%s`, run `glaze up %s`", word, word)
}

// usageError marks err as a usage error. The CLI library calls it for a flag or an argument that it cannot parse.
func usageError(_ context.Context, cmd *cli.Command, err error, isSubcommand bool) error {
	name := cmd.Name
	if isSubcommand {
		name = "glaze " + name
	}

	return fmt.Errorf("%w: %w (see `%s --help`)", errUsage, err, name)
}
