package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/term"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// ErrDifferences means that the session differs from the profile or does not run; the report is already on stdout.
var ErrDifferences = errors.New("the session differs from the profile")

// ActionDiff prints what up --clear would change in the running session, and changes nothing.
type ActionDiff struct {
	ActionBase
	tmux tmux.Client

	// out receives the report; it defaults to stdout and exists so tests can capture the output.
	out   io.Writer
	quiet bool

	// colour is true when stdout is a terminal that accepts colour.
	colour bool
}

// NewDiff returns the diff action for the profile and the tmux server of the command.
func NewDiff(cmd *cli.Command, logLevel string) (*ActionDiff, error) {
	base, err := NewActionBase(cmd, logLevel)
	if err != nil {
		return nil, err
	}

	tmuxClient, err := newTmuxClient(cmd, base.Logger)
	if err != nil {
		return nil, err
	}

	return &ActionDiff{
		ActionBase: *base,
		tmux:       tmuxClient,
		out:        os.Stdout,
		quiet:      cmd.Bool("quiet"),
		colour:     term.ColorEnabled(os.Stdout),
	}, nil
}

// Run compares the profile with the live session and writes the report once, after every tmux read succeeded.
// It returns ErrDifferences when up --clear has work to do, so the exit code answers a script.
func (a *ActionDiff) Run(ctx context.Context) error {
	a.tmux = a.tmux.WithContext(ctx)

	profile, diags := a.decodeProfile()
	if diags.HasErrors() {
		return a.DiagnosticsManager.Report(diags)
	}

	// The declared directories are read before the resolution fills the rest, so the report can say where a value came from.
	declared := emptyDirectories(profile)
	if err := profile.ResolveDirectories(a.profileDir()); err != nil {
		return err
	}

	if err := a.tmux.CheckVersion(); err != nil {
		return err
	}

	warnRename(a.Logger, "session", profile.Name, tmux.SanitizeSessionName(profile.Name))
	want := desiredSession(profile, declared)

	exists, err := a.tmux.HasSession(profile.Name)
	if err != nil {
		return fmt.Errorf("could not check for session `%s`: %w", profile.Name, err)
	}

	var p plan
	inside := false
	if exists {
		session, err := a.tmux.FindSessionByName(profile.Name)
		if err != nil {
			return fmt.Errorf("could not find session `%s`: %w", profile.Name, err)
		}

		live, err := captureSession(a.tmux, session, a.Logger)
		if err != nil {
			return err
		}

		p = compare(want, live)
		inside = a.insideSession(session)
	} else {
		if a.Parser.SessionNameUsesPath() {
			a.Logger.Info("the session name depends on the current directory; run diff from the directory that up ran in", "session", want.Name)
		}

		p = creationPlan(want)
	}

	if !a.quiet {
		var b bytes.Buffer
		if err := render(&b, p, inside, a.colour); err != nil {
			return err
		}

		if _, err := a.out.Write(b.Bytes()); err != nil {
			return err
		}
	}

	if p.adds+p.changes+p.removes > 0 {
		return ErrDifferences
	}

	return nil
}

// insideSession reports whether glaze runs in a pane of the compared session, which only chooses the wording of the hint.
func (a *ActionDiff) insideSession(session *tmux.Session) bool {
	current, err := a.tmux.CurrentSession()
	if err != nil || current == nil {
		return false
	}

	return current.Id == session.Id
}
