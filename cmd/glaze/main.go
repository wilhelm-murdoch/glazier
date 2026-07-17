package main

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
)

var (
	// Version describes the version of the current build.
	Version = "dev"

	// Commit describes the commit of the current build.
	Commit = "none"

	// Date describes the date of the current build.
	Date = "unknown"

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

// variableFlags returns fresh instances of the flags shared by every command
// that resolves a profile's variables: repeatable --var overrides and a
// --var-file of values. Fresh instances per command, since parsed flag state
// lives on the flag value itself.
func variableFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringSliceFlag{
			Name:      "var",
			Usage:     "set multiple variables in the form of \"key=value\"",
			Validator: validateVarFlags,
		},
		&cli.StringFlag{
			Name:  "var-file",
			Usage: "path to an HCL or JSON file of variable values",
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
	var logLevel string

	cli.VersionPrinter = func(ctx *cli.Command) {
		fmt.Printf("Version: %s, Stage: %s, Commit: %s, Date: %s\n", Version, Stage, Commit, Date)
	}

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Usage:   "print only the version",
		Aliases: []string{"v"},
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
					profilePathFlag(),
				}, socketFlags(), variableFlags()),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					action, err := actions.NewUp(cmd, logLevel)
					if err != nil {
						return err
					}

					return action.Run()
				},
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
				Action: func(ctx context.Context, cmd *cli.Command) error {
					action, err := actions.NewDown(cmd, logLevel)
					if err != nil {
						return err
					}

					return action.Run()
				},
			},
			{
				Name:  "ls",
				Usage: "list the sessions running on the target tmux server",
				Flags: socketFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					action, err := actions.NewLs(cmd, logLevel)
					if err != nil {
						return err
					}

					return action.Run()
				},
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
				Action: func(ctx context.Context, cmd *cli.Command) error {
					action, err := actions.NewFormat(cmd, logLevel)
					if err != nil {
						return err
					}

					return action.Run()
				},
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
				}, socketFlags()),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					action, err := actions.NewSave(cmd, logLevel)
					if err != nil {
						return err
					}

					return action.Run()
				},
			},
		},
	}

	// --var / --var-file values are arbitrary strings (tags, titles) that
	// routinely contain commas; disable the slice-flag comma split so a
	// value like `tags=one,two,three` arrives as one string, not three. The
	// separator config is read per owning command, so set it on each.
	for _, sub := range app.Commands {
		sub.DisableSliceFlagSeparator = true
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
