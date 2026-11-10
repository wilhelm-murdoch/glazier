package actions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// ActionLs lists the sessions on a tmux server, or the profiles in the profile registry.
type ActionLs struct {
	Command *cli.Command
	Logger  *logger.Logger
	tmux    tmux.Client

	// out receives the rendered table; it defaults to stdout and exists so
	// tests can capture the output.
	out io.Writer
}

// NewLs returns the ls action, which reads the tmux server and no profile. With --profiles it reads only the registry, so it needs no tmux.
func NewLs(cmd *cli.Command, logLevel string) (*ActionLs, error) {
	log := newLogger(cmd, logLevel)

	action := &ActionLs{
		Command: cmd,
		Logger:  log,
		out:     os.Stdout,
	}

	if cmd.Bool("profiles") {
		return action, nil
	}

	tmuxClient, err := newTmuxClient(cmd, log)
	if err != nil {
		return nil, err
	}

	action.tmux = tmuxClient

	return action, nil
}

// Run prints each session with its window count and directory, and marks the session of the pane that glaze runs in.
func (a *ActionLs) Run(ctx context.Context) error {
	if a.Command.Bool("profiles") {
		return a.listProfiles()
	}

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
		return err
	}

	table := a.table()
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

// listProfiles prints the name and the path of each profile in the registry; it reads only the directory and parses no profile.
func (a *ActionLs) listProfiles() error {
	warnIgnoredConfigHome(a.Logger)

	// Each registry error already names the cause and the variable to fix, so it is returned as it is.
	dir, err := files.RegistryDir()
	if err != nil {
		return err
	}

	profiles, skipped, err := files.ListProfiles(dir)
	if errors.Is(err, fs.ErrNotExist) {
		// No registry means no profiles, so stdout stays empty for a script that reads it.
		a.Logger.Info("no profile registry; `glaze save <name>` creates it", "path", dir)
		return nil
	}

	if err != nil {
		return err
	}

	for _, entry := range skipped {
		a.Logger.Warn("registry entry is not a profile", "file", entry.File, "reason", entry.Reason)
	}

	table := a.table()
	_, _ = fmt.Fprintln(table, "NAME\tPATH")

	for _, profile := range profiles {
		_, _ = fmt.Fprintf(table, "%s\t%s\n", profile.Name, profile.Path)
	}

	return table.Flush()
}

// table returns the writer of both listings; write errors surface on Flush, so the intermediate ones are ignored.
func (a *ActionLs) table() *tabwriter.Writer {
	return tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
}
