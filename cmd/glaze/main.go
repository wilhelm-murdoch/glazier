package main

import (
	"context"
	"fmt"
	"io"
	"net/mail"
	"os"
	"os/signal"
	"runtime/debug"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
)

var (
	// Version describes the version of the current build.
	Version = unstampedVersion

	// Commit describes the commit of the current build.
	Commit = unstampedCommit

	// Date describes the date of the current build.
	Date = unstampedDate

	// Release describes the stage of the current build, eg; development, production, etc...
	Stage = "unknown"
)

// validateVarFlags rejects --var values that do not follow the `key=value`
// format; it is shared by every command that accepts variables.
func validateVarFlags(value []string) error {
	for _, variable := range value {
		if !strings.Contains(variable, "=") {
			return fmt.Errorf(
				"the --var `%s` does not match the required format of `key=value`",
				variable,
			)
		}

		parts := strings.SplitN(variable, "=", 2)

		if strings.HasSuffix(parts[0], " ") {
			return fmt.Errorf(
				"the --var name `%s` appears to have trailing spaces and does not match the required format of `key=value`",
				parts[0],
			)
		}
	}

	return nil
}

// variableFlags returns new --var and --var-file flags for a command.
// Each command needs its own instances, because a flag keeps its parsed value.
func variableFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringSliceFlag{
			Name:      "var",
			Usage:     "set multiple variables in the form of \"key=value\"",
			Validator: validateVarFlags,
		},
		&cli.StringFlag{
			Name:  "var-file",
			Usage: "path to an HCL file of variable values",
		},
	}
}

// socketFlags returns fresh instances of the flags shared by every command
// that addresses a tmux server on a non-default socket.
func socketFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "socket-path",
			Usage: "optional path to the tmux socket",
		},
		&cli.StringFlag{
			Name:  "socket-name",
			Usage: "optional name for the tmux socket",
		},
	}
}

// profilePathFlag returns the flag naming the profile a command reads.
func profilePathFlag() cli.Flag {
	return &cli.StringFlag{
		Name:  "profile-path",
		Usage: "specify a path to a target glaze definition file",
	}
}

func main() {
	if info, ok := debug.ReadBuildInfo(); ok {
		fillFromBuildInfo(info)
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	stop := cancelOnSignal(cancel)

	code := run(ctx, os.Args, os.Stderr)

	stop()
	os.Exit(code)
}

// run executes glaze with args, writes any error to stderr and returns the exit code.
func run(ctx context.Context, args []string, stderr io.Writer) int {
	err := newApp().Run(ctx, args)

	// A signal ends the run with whatever error the stopped tmux command gives, so name the signal instead.
	if err != nil && ctx.Err() != nil {
		err = fmt.Errorf("%w: %w", context.Cause(ctx), err)
	}

	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s\n", err)
		return exitCode(err)
	}

	return exitOK
}

// cancelOnSignal cancels the run on the first SIGINT or SIGTERM, with the signal as the cause.
// A second signal gets the default action, so it stops glaze at once.
func cancelOnSignal(cancel context.CancelCauseFunc) (stop func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		if received, ok := <-signals; ok {
			signal.Reset(os.Interrupt, syscall.SIGTERM)
			cancel(signalError{signal: received.(syscall.Signal)})
		}
	}()

	return func() {
		signal.Stop(signals)
		close(signals)
	}
}

// action is a glaze subcommand, built from the flags and then run.
type action interface {
	Run(ctx context.Context) error
}

// actionFor returns the CLI action that builds a subcommand with build and runs it.
// logLevel is a pointer, because the global flag is parsed after newApp returns.
func actionFor[A action](build func(*cli.Command, string) (A, error), logLevel *string) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		a, err := build(cmd, *logLevel)
		if err != nil {
			return err
		}

		return a.Run(ctx)
	}
}

// newApp returns the glaze command with all of its subcommands.
func newApp() *cli.Command {
	var logLevel string

	cli.VersionPrinter = func(ctx *cli.Command) {
		fmt.Printf("Version: %s, Stage: %s, Commit: %s, Date: %s\n", Version, Stage, Commit, Date)
	}

	// The flag belongs to glaze itself, so `glaze up -v` is a usage error and does not start a session.
	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Usage:   "print only the version",
		Aliases: []string{"v"},
		Local:   true,
	}

	currentYear, _, _ := time.Now().Date()

	app := &cli.Command{
		Name:    "glaze",
		Usage:   "easily manage tmux sessions, windows and panes",
		Version: Version,
		Authors: []any{
			mail.Address{Name: "Wilhelm Murdoch", Address: "wilhelm@devilmayco.de"},
		},
		Copyright: fmt.Sprintf(`(c) %d Wilhelm Codes ( https://wilhelm.codes )`, currentYear),
		// glaze picks the exit code itself, so the CLI library must not exit the process.
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		OnUsageError:   usageError,
		Action:         rootAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "log-level",
				Value:       "info",
				Usage:       "specify a log level",
				Destination: &logLevel,
				Validator: func(value string) error {
					if _, ok := logger.FriendlyToInternal[value]; !ok {
						return fmt.Errorf("specified an invalid log level value: %s", value)
					}

					return nil
				},
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "up",
				Usage: "apply the specified glaze profile",
				Flags: slices.Concat([]cli.Flag{
					&cli.BoolFlag{
						Name:  "detached",
						Usage: "start a tmux session using glaze in detached mode",
					},
					&cli.BoolFlag{
						Name:  "clear",
						Usage: "if it exists, clear the current glaze session before starting",
					},
					&cli.BoolFlag{
						Name:  "debug",
						Usage: "prints a list of all commands sent to the specified tmux socket",
					},
					&cli.BoolFlag{
						Name:  "keep-on-failure",
						Usage: "keep a partly built session when up fails, for debugging",
					},
					&cli.DurationFlag{
						Name:  "command-timeout",
						Usage: "stop waiting for the commands of a pane after this duration, for example 5m (0 waits with no limit)",
					},
					profilePathFlag(),
				}, socketFlags(), variableFlags()),
				Action: actionFor(actions.NewUp, &logLevel),
			},
			{
				Name:  "down",
				Usage: "kill the session described by the specified glaze profile",
				Flags: slices.Concat([]cli.Flag{
					&cli.StringFlag{
						Name:  "session",
						Usage: "name of the session to kill (defaults to the resolved profile's session)",
					},
					profilePathFlag(),
				}, socketFlags(), variableFlags()),
				Action: actionFor(actions.NewDown, &logLevel),
			},
			{
				Name:   "ls",
				Usage:  "list the sessions running on the target tmux server",
				Flags:  socketFlags(),
				Action: actionFor(actions.NewLs, &logLevel),
			},
			{
				Name:  "format",
				Usage: "rewrites the target glaze profile file to a canonical format",
				Flags: slices.Concat([]cli.Flag{
					&cli.BoolFlag{
						Name:  "stdout",
						Usage: "writes the formatted glaze output to your terminal",
					},
					&cli.BoolFlag{
						Name:  "validate",
						Usage: "validates the given glaze definition file and returns any diagnostics",
					},
					profilePathFlag(),
				}, variableFlags()),
				Action: actionFor(actions.NewFormat, &logLevel),
			},
			{
				Name:  "save",
				Usage: "running this within a tmux session will save its current state to the specified glaze profile",
				Flags: slices.Concat([]cli.Flag{
					&cli.StringFlag{
						Name:  "profile-path",
						Usage: "path the saved glaze definition file should be written to (defaults to .glaze)",
					},
					&cli.StringFlag{
						Name:  "session",
						Usage: "name of the session to save (defaults to the current session)",
					},
					&cli.BoolFlag{
						Name:  "stdout",
						Usage: "writes the saved glaze output to your terminal instead of a file",
					},
					&cli.BoolFlag{
						Name:  "force",
						Usage: "replaces the file at --profile-path when it exists",
					},
				}, socketFlags()),
				Action: actionFor(actions.NewSave, &logLevel),
			},
		},
	}

	// A --var value such as `tags=one,two` can contain commas, so turn off the comma split on every command.
	for _, sub := range app.Commands {
		sub.DisableSliceFlagSeparator = true
		sub.OnUsageError = usageError
	}

	return app
}
