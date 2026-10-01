package actions

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// ActionLs lists the sessions on a tmux server.
type ActionLs struct {
	Command *cli.Command
	Logger  *logger.Logger
	tmux    tmux.Client

	// out receives the rendered table; it defaults to stdout and exists so
	// tests can capture the output.
	out io.Writer
}

// NewLs returns the ls action, which reads the tmux server and no profile.
func NewLs(cmd *cli.Command, logLevel string) (*ActionLs, error) {
	log := newLogger(cmd, logLevel)

	tmuxClient, err := newTmuxClient(cmd, log)
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

// Run prints each session with its window count and directory, and marks the session of the pane that glaze runs in.
func (a *ActionLs) Run(ctx context.Context) error {
	a.tmux = a.tmux.WithContext(ctx)

	running, err := a.tmux.IsRunning()
	if err != nil {
		return err
	}

	// No server means no sessions, so stdout stays empty for a script that reads it.
	if !running {
		a.Logger.Info("no tmux server is running")
		return nil
	}

	sessions, err := a.tmux.Sessions()
	if err != nil {
		return fmt.Errorf("could not list sessions: %w", err)
	}

	// Only a pane of this server has a current session; elsewhere nothing gets a marker.
	currentSession, err := a.tmux.CurrentSession()
	if err != nil {
		return fmt.Errorf("could not determine current session: %w", err)
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
		if currentSession != nil && session.Id == currentSession.Id {
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
