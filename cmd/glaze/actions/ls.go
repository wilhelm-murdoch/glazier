package actions

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// ActionLs is a struct that represents a Glazier "action".
type ActionLs struct {
	Command *cli.Command
	Logger  *logger.Logger
	tmux    *tmux.Client

	// out receives the rendered table; it defaults to stdout and exists so
	// tests can capture the output.
	out io.Writer
}

// NewLs is responsible for creating a new ActionLs struct value. Like save,
// ls only inspects the running tmux server, so it does not resolve or parse
// a profile file.
func NewLs(cmd *cli.Command, logLevel string) (*ActionLs, error) {
	log := logger.New(logger.FriendlyToInternal[logLevel])

	tmuxClient, err := tmux.NewClient(
		cmd.String("socket-path"),
		cmd.String("socket-name"),
		log.Logger,
	)
	if err != nil {
		return nil, err
	}

	return &ActionLs{
		Command: cmd,
		Logger:  log,
		tmux:    tmuxClient,
		out:     os.Stdout,
	}, nil
}

// Run lists every session on the target tmux server with its window count
// and starting directory. The session the current client is attached to, if
// any, is marked with an asterisk.
func (a *ActionLs) Run() error {
	if !a.tmux.IsRunning() {
		return fmt.Errorf("no running tmux server found")
	}

	sessions, err := a.tmux.Sessions()
	if err != nil {
		return fmt.Errorf("could not list sessions: %w", err)
	}

	// Resolving the attached session only makes sense from inside tmux;
	// elsewhere `display-message` would report an arbitrary session.
	var current string
	if os.Getenv("TMUX") != "" {
		if name, err := a.tmux.CurrentSessionName(); err == nil {
			current = name
		}
	}

	// Write errors surface on Flush, so the intermediate ones are ignored.
	table := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "NAME\tWINDOWS\tPATH")

	for _, session := range sessions {
		windows, err := a.tmux.Windows(session)
		if err != nil {
			return fmt.Errorf(
				"could not list windows for session `%s`: %w",
				session.Name,
				err,
			)
		}

		marker := ""
		if session.Name == current {
			marker = "*"
		}

		_, _ = fmt.Fprintf(
			table,
			"%s%s\t%d\t%s\n",
			session.Name,
			marker,
			len(windows),
			session.StartingDirectory,
		)
	}

	return table.Flush()
}
