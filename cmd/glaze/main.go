package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
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
	"github.com/wilhelm-murdoch/glazier/pkg/files"
)

// The help template has no section for arguments, so the descriptions explain the positional name.
const (
	registryDescription = "A name selects <name>.glaze in the profile registry: $GLAZE_REGISTRY, else $XDG_CONFIG_HOME/glaze, else ~/.config/glaze.\n" +
		"Without a name, glaze uses --profile-path, ./.glaze or $GLAZE_PATH/.glaze."
	diffDescription = "diff compares the profile with the session that runs under its name. It prints what `glaze up --clear` would change. It reads the session and changes nothing.\n" +
		"A `+` line is a window or pane that the profile has and the session does not have. A `-` line is a window or pane that the session has and the profile does not have. " +
		"A `~` line is a value that differs and that `up --clear` would set. A `?` line is a value that differs, but tmux cannot tell if glaze set it or a user changed it. A `?` line does not change the exit code.\n" +
		"diff compares the session directory, the windows by name, the panes by position, the window order and the focus that the profile declares. " +
		"It does not compare commands, envs, hooks and options, because tmux does not report what glaze set. It does not compare size, adjust and layout, because tmux reports them in a form that changes with the terminal size.\n" +
		"Exit code 0 means no counted difference. Exit code 5 means a difference or a session that does not run.\n"
	saveDescription = "A name writes <name>.glaze into the profile registry: $GLAZE_REGISTRY, else $XDG_CONFIG_HOME/glaze, else ~/.config/glaze. It creates the profile registry when it is missing.\n" +
		"Without a name, save writes --profile-path or ./.glaze."
)

var (
	// Version describes the version of the current build.
	Version = unstampedVersion

	// Commit describes the commit of the current build.
	Commit = unstampedCommit

	// Date describes the date of the current build.
	Date = unstampedDate

	// Stage describes the stage of the current build, for example development or production.
	Stage = "unknown"
)

// validateVarFlags rejects --var values that do not follow the `key=value`
// format; it is shared by every command that accepts variables.
func validateVarFlags(value []string) error {
	for _, variable := range value {
		key, _, ok := strings.Cut(variable, "=")
		if !ok {
			return fmt.Errorf(
				"the --var `%s` does not match the required format of `key=value`",
				variable,
			)
		}

		if strings.HasSuffix(key, " ") {
			return fmt.Errorf(
				"the --var name `%s` appears to have trailing spaces and does not match the required format of `key=value`",
				key,
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
			Usage:     "set a variable as key=value; repeat the flag for each variable",
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
		Usage: "specify the path to a profile",
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

	// The report of diff already said that the session differs, so that result gets no message.
	if err != nil && !errors.Is(err, actions.ErrDifferences) {
		_, _ = fmt.Fprintln(stderr, err)
	}

	if err != nil {
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

// actionFor returns the CLI action that checks the positional words, builds a subcommand with build and runs it.
// logLevel is a pointer, because the global flag is parsed after newApp returns.
func actionFor[A action](build func(*cli.Command, string) (A, error), logLevel *string, takesName bool) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		if err := checkArgs(cmd, takesName); err != nil {
			return err
		}

		a, err := build(cmd, *logLevel)
		if err != nil {
			return err
		}

		return a.Run(ctx)
	}
}

// checkArgs rejects a word that the command does not take, a word that is not a profile name, a second word and a name next to --profile-path or --session.
// It runs inside the Action, which the library never calls in completion mode, so a bad word cannot break a Tab.
func checkArgs(cmd *cli.Command, takesName bool) error {
	// A command built outside the library, for example by a test, has no Args at all.
	args := cmd.Args()
	if args == nil || !args.Present() {
		return nil
	}

	help := fmt.Sprintf("(see `glaze %s --help`)", cmd.Name)
	if !takesName {
		return fmt.Errorf("%w: `glaze %s` takes no argument; got `%s` %s", errUsage, cmd.Name, args.First(), help)
	}

	// The shape comes first: a word such as `-1` ends the flag parsing, so a flag after it is not the cause.
	if err := files.ValidateProfileName(args.First()); err != nil {
		return fmt.Errorf("%w: %w %s", errUsage, err, help)
	}

	if args.Len() > 1 {
		return fmt.Errorf("%w: `glaze %s` takes one name; got `%s` and `%s` %s", errUsage, cmd.Name, args.Get(0), args.Get(1), help)
	}

	// The value and not IsSet, so `--profile-path=` with nothing after it does not count as a path.
	if cmd.String("profile-path") != "" {
		return fmt.Errorf("%w: give a name or --profile-path, not both %s", errUsage, help)
	}

	// On down the flag and the name select the same thing; on save the flag names the session and the name names the file.
	if cmd.Name == "down" && cmd.String("session") != "" {
		return fmt.Errorf("%w: give a name or --session, not both %s", errUsage, help)
	}

	return nil
}

// newApp returns the glaze command with all of its subcommands.
func newApp() *cli.Command {
	var logLevel string

	cli.VersionPrinter = func(ctx *cli.Command) {
		_, _ = fmt.Fprintf(ctx.Root().Writer, "Version: %s, Stage: %s, Commit: %s, Date: %s\n", Version, Stage, Commit, Date)
	}

	// The flag belongs to glaze itself, so `glaze up -v` is a usage error and does not start a session.
	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Usage:   "print the version, the stage, the commit and the build date",
		Aliases: []string{"v"},
		Local:   true,
	}

	app := &cli.Command{
		Name:    "glaze",
		Usage:   "manage tmux sessions, windows and panes from a profile",
		Version: Version,
		// A plain string, because the help template prints an author with %v, which shows a struct field by field.
		Authors:   []any{"Wilhelm Murdoch <wilhelm@devilmayco.de>"},
		Copyright: fmt.Sprintf(`(c) %d Wilhelm Codes ( https://wilhelm.codes )`, time.Now().Year()),
		// glaze picks the exit code itself, so the CLI library must not exit the process.
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		OnUsageError:   usageError,
		Action:         rootAction,
		// The library hides its `completion` command; glaze shows it, so `--help` tells a user that it exists.
		EnableShellCompletion:           true,
		ConfigureShellCompletionCommand: configureCompletion,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "log-level",
				Value:       "info",
				Usage:       "set the log level: trace, debug, info, warning, error or critical",
				Destination: &logLevel,
				Validator: func(value string) error {
					if _, ok := logger.FriendlyToInternal[value]; !ok {
						return fmt.Errorf("`%s` is not a log level; use one of %s", value, strings.Join(slices.Sorted(maps.Keys(logger.FriendlyToInternal)), ", "))
					}

					return nil
				},
			},
		},
		Commands: []*cli.Command{
			{
				Name:          "up",
				Usage:         "apply the specified glaze profile",
				ArgsUsage:     "[name]",
				Description:   registryDescription,
				ShellComplete: completeProfileNames,
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
						Usage: "print each command sent to the tmux socket and the text of each pane and session command",
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
				Action: actionFor(actions.NewUp, &logLevel, true),
			},
			{
				Name:          "diff",
				Usage:         "show what up --clear would change in the running session",
				ArgsUsage:     "[name]",
				Description:   diffDescription + registryDescription,
				ShellComplete: completeProfileNames,
				Flags: slices.Concat([]cli.Flag{
					&cli.BoolFlag{
						Name:    "quiet",
						Aliases: []string{"q"},
						Usage:   "print nothing; the exit code carries the result",
					},
					profilePathFlag(),
				}, socketFlags(), variableFlags()),
				Action: actionFor(actions.NewDiff, &logLevel, true),
			},
			{
				Name:          "down",
				Usage:         "kill the session described by the specified glaze profile",
				ArgsUsage:     "[name]",
				Description:   registryDescription,
				ShellComplete: completeProfileNames,
				Flags: slices.Concat([]cli.Flag{
					&cli.StringFlag{
						Name:  "session",
						Usage: "name of the session to kill (defaults to the resolved profile's session)",
					},
					profilePathFlag(),
				}, socketFlags(), variableFlags()),
				Action: actionFor(actions.NewDown, &logLevel, true),
			},
			{
				Name:  "ls",
				Usage: "list the sessions running on the target tmux server",
				Flags: slices.Concat([]cli.Flag{
					&cli.BoolFlag{
						Name:  "profiles",
						Usage: "list the profiles in the profile registry instead of the sessions",
					},
				}, socketFlags()),
				Action: actionFor(actions.NewLs, &logLevel, false),
			},
			{
				Name:          "format",
				Usage:         "rewrite a profile in the canonical format",
				ArgsUsage:     "[name]",
				Description:   registryDescription,
				ShellComplete: completeProfileNames,
				Flags: slices.Concat([]cli.Flag{
					&cli.BoolFlag{
						Name:  "stdout",
						Usage: "writes the formatted profile to your terminal",
					},
					&cli.BoolFlag{
						Name:  "validate",
						Usage: "validates the given profile and returns any diagnostics",
					},
					profilePathFlag(),
				}, variableFlags()),
				Action: actionFor(actions.NewFormat, &logLevel, true),
			},
			{
				Name:        "save",
				Usage:       "write the structure of a running session as a profile",
				ArgsUsage:   "[name]",
				Description: saveDescription,
				Flags: slices.Concat([]cli.Flag{
					&cli.StringFlag{
						Name:  "profile-path",
						Usage: "path the saved profile should be written to (defaults to .glaze)",
					},
					&cli.StringFlag{
						Name:  "session",
						Usage: "name of the session to save (defaults to the current session)",
					},
					&cli.BoolFlag{
						Name:  "stdout",
						Usage: "writes the saved profile to your terminal instead of a file",
					},
					&cli.BoolFlag{
						Name:  "force",
						Usage: "replaces the target profile when it exists",
					},
				}, socketFlags()),
				Action: actionFor(actions.NewSave, &logLevel, true),
			},
		},
	}

	// A --var value such as `tags=one,two` can contain commas, so turn off the comma split on every command.
	// A profile can be named help, so no leaf gets a help subcommand; --help and `glaze help <command>` still work.
	for _, sub := range app.Commands {
		sub.DisableSliceFlagSeparator = true
		sub.OnUsageError = usageError
		sub.HideHelpCommand = true
	}

	return app
}
