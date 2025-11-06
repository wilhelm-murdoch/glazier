package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions/format"
	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions/save"
	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions/up"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/internal/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
)

const defaultErrCode = 1

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

func main() {
	logger := logger.New(slog.LevelDebug)

	cli.VersionPrinter = func(ctx *cli.Context) {
		fmt.Printf("Version: %s, Stage: %s, Commit: %s, Date: %s\n", Version, Stage, Commit, Date)
	}

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Usage:   "print only the version",
		Aliases: []string{"v"},
	}

	currentYear, _, _ := time.Now().Date()

	app := &cli.App{
		Name:     "glaze",
		Usage:    "easily manage tmux sessions, windows and panes",
		Version:  Version,
		Compiled: time.Now(),
		Authors: []*cli.Author{{
			Name:  "Wilhelm Murdoch",
			Email: "wilhelm@devilmayco.de",
		}},
		Copyright: fmt.Sprintf(`(c) %d Wilhelm Codes ( https://wilhelm.codes )`, currentYear),
		Before: func(ctx *cli.Context) error {
			ok, path := tmux.IsInstalled()

			if !ok {
				return fmt.Errorf(
					"could not find a valid tmux installation on path: %s",
					path,
				)
			}

			return nil
		},
		Commands: []*cli.Command{{
			Name:  "up",
			Usage: "apply the specified glaze profile",
			Flags: []cli.Flag{
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
				&cli.StringFlag{
					Name:  "socket-path",
					Value: "",
					Usage: "optional path to the tmux socket",
					Action: func(ctx *cli.Context, value string) error {
						if ctx.String("socket-name") != "" && value != "" {
							return cli.Exit(
								"cannot specify both --socket-name and --socket-path flags",
								defaultErrCode,
							)
						}

						if value != "" && !files.FileExists(value) {
							return cli.Exit(
								fmt.Sprintf("specified --socket-path of %s does not exist", value),
								defaultErrCode,
							)
						}

						return nil
					},
				},
				&cli.StringFlag{
					Name:  "profile-path",
					Value: "",
					Usage: "specify a path to a target glaze definition file",
				},
				&cli.StringFlag{
					Name:  "socket-name",
					Value: "",
					Usage: "optional name for the tmux socket",
					Action: func(ctx *cli.Context, value string) error {
						if ctx.String("socket-path") != "" && value != "" {
							return cli.Exit(
								"cannot specify both --socket-name and --socket-path flags",
								defaultErrCode,
							)
						}

						return nil
					},
				},
				&cli.StringSliceFlag{
					Name:  "var",
					Usage: "set multiple variables in the form of \"key=value\"",
					Action: func(ctx *cli.Context, value []string) error {
						for _, variable := range value {
							if !strings.Contains(variable, "=") {
								return cli.Exit(
									fmt.Sprintf(
										"the --var `%s` does not match the required format of `key=value`",
										variable,
									),
									defaultErrCode,
								)
							}

							parts := strings.SplitN(variable, "=", 2)

							if strings.HasSuffix(parts[0], " ") {
								return cli.Exit(
									fmt.Sprintf(
										"the --var name `%s` appears to have trailing spaces and does not match the required format of `key=value`",
										parts[0],
									),
									defaultErrCode,
								)
							}
						}

						return nil
					},
				},
			},
			Action: func(ctx *cli.Context) error {
				action, err := up.NewAction(ctx, logger)
				if err != nil {
					return err
				}

				return action.Run()
			},
		}, {
			Name:  "format",
			Usage: "rewrites the target glaze profile file to a canonical format",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "stdout",
					Usage: "writes the formatted glaze output to your terminal",
				},
				&cli.BoolFlag{
					Name:  "validate",
					Usage: "validates the given glaze definition file and returns any diagnostics",
				},
			},
			Action: func(ctx *cli.Context) error {
				action, err := format.NewAction(ctx, logger)
				if err != nil {
					return err
				}

				return action.Run()
			},
		}, {
			Name:  "save",
			Usage: "running this within a tmux session will save its current state to the specified glaze profile",
			Action: func(ctx *cli.Context) error {
				action, err := save.NewAction(ctx, logger)
				if err != nil {
					return err
				}

				return action.Run()
			},
		}},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Glaze %v\n", err)
		os.Exit(1)
	}
}
