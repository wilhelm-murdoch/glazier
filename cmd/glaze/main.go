package main

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions/format"
	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions/save"
	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions/up"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
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
	log := logger.New(logger.LevelTrace)

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
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			ok, path := tmux.IsInstalled()

			if !ok {
				return ctx, fmt.Errorf(
					"could not find a valid tmux installation on path: %s",
					path,
				)
			}

			return ctx, nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "log-level",
				Value: logger.LevelTraceLabel,
				Usage: "specify a log level",
				Validator: func(value string) error {
					log.Info("dropping some logs")
					//	log = logger.New(logger.LevelTrace)
					return nil
				},
			},
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
					Action: func(ctx context.Context, cmd *cli.Command, value string) error {
						if cmd.String("socket-name") != "" && value != "" {
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
					Action: func(ctx context.Context, cmd *cli.Command, value string) error {
						if cmd.String("socket-path") != "" && value != "" {
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
					Action: func(ctx context.Context, cmd *cli.Command, value []string) error {
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
			Action: func(ctx context.Context, cmd *cli.Command) error {
				action, err := up.NewAction(cmd, log)
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
			Action: func(ctx context.Context, cmd *cli.Command) error {
				action, err := format.NewAction(cmd, log)
				if err != nil {
					return err
				}

				return action.Run()
			},
		}, {
			Name:  "save",
			Usage: "running this within a tmux session will save its current state to the specified glaze profile",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				action, err := save.NewAction(cmd, log)
				if err != nil {
					return err
				}

				return action.Run()
			},
		}},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Error("Glaze exited with an error", "err", err)
		os.Exit(1)
	}
}
