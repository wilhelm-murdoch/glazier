package actions

import (
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// ActionDown is a struct that represents a Glazier "action".
type ActionDown struct {
	Command *cli.Command
	Logger  *logger.Logger
	tmux    *tmux.Client

	// base is only populated when the session name has to come from a
	// profile; `down --session <name>` needs no profile at all.
	base *ActionBase
}

// NewDown is responsible for creating a new ActionDown struct value. The
// profile is only resolved and parsed when no --session override is given,
// since it is then the sole source of the session name.
func NewDown(cmd *cli.Command, logLevel string) (*ActionDown, error) {
	log := logger.New(logger.FriendlyToInternal[logLevel])

	tmuxClient, err := tmux.NewClient(
		cmd.String("socket-path"),
		cmd.String("socket-name"),
		log.Logger,
	)
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
			return nil, err
		}

		action.base = base
	}

	return action, nil
}

// Run kills the session named by --session or, failing that, by the resolved
// profile. Tearing down a session that is not running is a no-op rather than
// an error so `down` stays idempotent for scripts, mirroring how `up` treats
// an already-running session.
func (a *ActionDown) Run() error {
	name, err := a.sessionName()
	if err != nil {
		return err
	}

	if !a.tmux.HasSession(name) {
		a.Logger.Info("nothing to do; session is not running", "session", name)
		return nil
	}

	if err := a.tmux.KillSessionByName(name); err != nil {
		return fmt.Errorf("could not bring down session `%s`: %w", name, err)
	}

	a.Logger.Info("session killed", "session", name)

	return nil
}

// sessionName resolves the name of the session to tear down: the --session
// flag wins, otherwise only the profile's `name` attribute is evaluated so
// interpolated names (e.g. `name = "gig-${district}"`) resolve exactly as they
// did for `up`. The rest of the profile (windows, panes, their commands) is
// never evaluated, so variables used only deeper in the tree are not required
// to bring a session down.
func (a *ActionDown) sessionName() (string, error) {
	if name := a.Command.String("session"); name != "" {
		return name, nil
	}

	// requireAll is false: `down` evaluates only the session name, so a
	// variable that is required deeper in the profile but never referenced by
	// `name` must not block a teardown (see DecodeSessionName).
	ctx, ctxDiags := a.base.Parser.VariableContext(a.Command.StringSlice("var"), a.Command.String("var-file"), false)
	if ctxDiags.HasErrors() {
		a.base.DiagnosticsManager.Extend(ctxDiags)
		return "", a.base.DiagnosticsManager.Write()
	}

	name, diags := a.base.Parser.DecodeSessionName(ctx)
	if diags.HasErrors() {
		a.base.DiagnosticsManager.Extend(diags)
		return "", a.base.DiagnosticsManager.Write()
	}

	return name, nil
}
