package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// The exit codes of glaze. The README lists them, so a change here needs a change there.
const (
	exitOK             = 0
	exitFailure        = 1
	exitUsage          = 2
	exitInvalidProfile = 3
	exitUnreachable    = 4
)

// errUsage marks an error in the command line itself, for example an unknown flag.
var errUsage = errors.New("usage error")

// exitCode returns the exit code for the error that a command returned.
func exitCode(err error) int {
	var exitCoder cli.ExitCoder

	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, errUsage), errors.As(err, &exitCoder):
		// The CLI library returns an ExitCoder only for an unknown command.
		return exitUsage
	case errors.Is(err, diagnostics.ErrHasDiagnostics), errors.Is(err, files.ErrProfileNotFound):
		return exitInvalidProfile
	case errors.Is(err, tmux.ErrUnreachable):
		return exitUnreachable
	default:
		return exitFailure
	}
}

// usageError marks err as a usage error. The CLI library calls it for a flag or an argument that it cannot parse.
func usageError(_ context.Context, cmd *cli.Command, err error, isSubcommand bool) error {
	name := cmd.Name
	if isSubcommand {
		name = "glaze " + name
	}

	return fmt.Errorf("%w: %w (see `%s --help`)", errUsage, err, name)
}
