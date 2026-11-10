package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/internal/spec"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// ActionDown kills the session of a profile.
type ActionDown struct {
	Command *cli.Command
	Logger  *logger.Logger
	tmux    tmux.Client

	// base is only populated when the session name has to come from a
	// profile; `down --session <name>` needs no profile at all.
	base *ActionBase
}

// NewDown returns the down action. It reads the profile only without --session, because then the profile names the session.
func NewDown(cmd *cli.Command, logLevel string) (*ActionDown, error) {
	log := newLogger(cmd, logLevel)

	tmuxClient, err := newTmuxClient(cmd, log)
	if err != nil {
		return nil, err
	}

	action := &ActionDown{
		Command: cmd,
		Logger:  log,
		tmux:    tmuxClient,
	}

	if cmd.String("session") == "" {
		base, err := NewActionBase(cmd, logLevel)
		if err != nil {
			// A registry profile can be gone while its session still runs, so name the way to kill it.
			if profileName(cmd) != "" && errors.Is(err, files.ErrProfileNotFound) {
				return nil, fmt.Errorf("%w\n - to kill a session by its tmux name, use --session", err)
			}

			return nil, err
		}

		action.base = base
	}

	return action, nil
}

// Run kills the session that --session or the profile names.
// A session that does not run is not an error, so `down` is safe to repeat in a script.
func (a *ActionDown) Run(ctx context.Context) error {
	a.tmux = a.tmux.WithContext(ctx)

	name, err := a.sessionName()
	if err != nil {
		return err
	}

	exists, err := a.tmux.HasSession(name)
	if err != nil {
		return fmt.Errorf("could not check for session `%s`: %w", name, err)
	}

	if !exists {
		// A name built from path.* changes with the directory, so say how to reach the session from elsewhere.
		if a.base != nil && a.base.Parser.SessionNameUsesPath() {
			a.Logger.Info("nothing to do; session is not running; the name depends on the current directory, so from another directory use --session", "session", name)
			return nil
		}

		a.Logger.Info("nothing to do; session is not running", "session", name)
		return nil
	}

	if err := a.tmux.KillSessionByName(name); err != nil {
		return fmt.Errorf("could not bring down session `%s`: %w", name, err)
	}

	a.Logger.Info("session killed", "session", name)

	return nil
}

// sessionName returns --session, or else the evaluated `name` of the profile.
// Only `name` is evaluated, so a variable that only windows and panes use needs no value.
func (a *ActionDown) sessionName() (string, error) {
	if name := a.Command.String("session"); name != "" {
		return name, nil
	}

	// requireAll is false, so a variable that only windows and panes use does not block `down`.
	ctx, ctxDiags := a.base.Parser.VariableContext(a.Command.StringSlice("var"), a.Command.String("var-file"), false)
	if ctxDiags.HasErrors() {
		return "", a.base.DiagnosticsManager.Report(ctxDiags)
	}

	// A nameless registry profile stops here, before any session is looked up.
	if diags := a.base.Parser.CheckRegistryName(a.base.ProfileName, ctx); diags.HasErrors() {
		return "", a.base.DiagnosticsManager.Report(diags)
	}

	name, diags := a.base.Parser.DecodeSessionName(spec.SessionName, ctx)
	if diags.HasErrors() {
		return "", a.base.DiagnosticsManager.Report(diags)
	}

	return name, nil
}
