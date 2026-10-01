package actions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// ActionUp is a struct that represents a Glazier "action".
type ActionUp struct {
	ActionBase
	tmux    *tmux.Client
	session *tmux.Session

	// optionTables tells glaze which scope tmux keeps each option in.
	optionTables tmux.OptionTables

	// windowDefaults holds the window options declared on the session, which apply to every window.
	windowDefaults map[string]string

	// runner runs pane and session commands. It is created when the first command runs.
	runner *tmux.CommandRunner

	// commandTimeout limits how long glaze waits for the commands of one pane. Zero waits with no limit.
	commandTimeout time.Duration
}

// NewUp is responsible for creating a new ActionFormat struct value pre-populated
// with fields that are common across all other action structs as well as a tmux
// client.
func NewUp(cmd *cli.Command, logLevel string) (*ActionUp, error) {
	base, err := NewActionBase(cmd, logLevel)
	if err != nil {
		return nil, err
	}

	tmuxClient, err := tmux.NewClient(
		cmd.String("socket-path"),
		cmd.String("socket-name"),
		base.Logger.Logger,
	)
	if err != nil {
		return nil, err
	}

	return &ActionUp{
		ActionBase:     *base,
		tmux:           tmuxClient,
		commandTimeout: cmd.Duration("command-timeout"),
	}, nil
}

// Run decodes the profile, then creates and provisions its session, or attaches to the session when it already runs.
// A run that fails or that ctx cancels removes the session that it created.
func (a *ActionUp) Run(ctx context.Context) error {
	client := a.tmux.WithContext(ctx)
	a.tmux = &client

	profile, err := a.loadProfile()
	if err != nil {
		return err
	}

	existed, err := a.resolveSession(profile)
	if err != nil {
		return err
	}

	// A pre-existing session is left untouched: resolveSession has already
	// attached to it when not detached. Re-provisioning would duplicate
	// windows and panes, so rebuilding an existing session is opt-in via
	// --clear (which kills it first, so it is treated as new here).
	if existed {
		return nil
	}

	if err := a.provisionSession(profile); err != nil {
		a.rollBack()
		return fmt.Errorf("failed to provision session `%s`: %w", a.session.Name, err)
	}

	return a.attachToSession()
}

// rollBack removes the session that this run created, unless --keep-on-failure keeps it for debugging.
func (a *ActionUp) rollBack() {
	if a.Command.Bool("keep-on-failure") {
		a.Logger.Warn("kept the partly built session; `glaze up --clear` rebuilds it", "session", a.session.Name)
		return
	}

	// The run can end because of a signal, so the clean-up must not use the cancelled context.
	session := *a.session
	session.Client = session.Client.WithoutCancel()

	if err := session.Kill(); err != nil {
		a.Logger.Warn("could not remove the partly built session", "session", session.Name, "error", err)
		return
	}

	a.Logger.Info("removed the partly built session", "session", session.Name)
}

// attachToSession handles attaching the tmux client to the newly created session.
func (a *ActionUp) attachToSession() error {
	if a.Command.Bool("detached") {
		return nil
	}

	err := a.tmux.Attach(a.session)
	if errors.Is(err, tmux.ErrOtherServer) {
		a.Logger.Info(
			"glaze runs inside another tmux server, so it does not attach",
			"session", a.session.Name,
			"attach", a.tmux.AttachCommand(a.session),
		)

		return nil
	}

	return err
}

// provisionSession creates the windows and panes as defined in the profile.
func (a *ActionUp) provisionSession(profile *decoders.Session) error {
	tables, err := a.tmux.OptionTables()
	if err != nil {
		return fmt.Errorf("could not read the tmux option tables: %w", err)
	}

	a.optionTables = tables

	if err := a.applySessionSettings(profile); err != nil {
		return err
	}

	first, err := a.getFirstWindow(a.session)
	if err != nil {
		return err
	}

	if err := a.generateWindows(profile.Windows, first); err != nil {
		return err
	}

	// Session commands run in the session's active pane, once all windows and panes exist.
	if len(profile.Commands) > 0 {
		pane, err := a.session.ActivePane()
		if err != nil {
			return err
		}

		if err := a.runCommands("session", a.session.Name, pane, profile.Commands); err != nil {
			return fmt.Errorf("could not run the commands for session `%s`: %w", a.session.Name, err)
		}
	}

	return nil
}

// applySessionSettings applies the environment variables and hooks defined on
// the session block to the resolved tmux session.
func (a *ActionUp) applySessionSettings(profile *decoders.Session) error {
	if a.session == nil {
		return nil
	}

	for key, value := range profile.Envs {
		a.Logger.Info("setting session env", "key", key, "session", a.session.Name)
		if err := a.session.SetEnv(key, value); err != nil {
			return fmt.Errorf(
				"could not set env `%s` on session `%s`: %w",
				key,
				a.session.Name,
				err,
			)
		}
	}

	for hook, command := range profile.Hooks {
		a.Logger.Info("setting session hook", "hook", hook, "session", a.session.Name)
		if err := a.session.SetHook(hook, command); err != nil {
			return fmt.Errorf(
				"could not set hook `%s` on session `%s`: %w",
				hook,
				a.session.Name,
				err,
			)
		}
	}

	for option, value := range profile.Options {
		// A window or pane option declared on the session applies to every window that glaze creates.
		if a.optionTables.IsWindowOnly(option) {
			if a.windowDefaults == nil {
				a.windowDefaults = make(map[string]string)
			}

			a.windowDefaults[option] = value
			continue
		}

		a.Logger.Info("setting session option", "option", option, "session", a.session.Name)
		if err := a.session.SetOption(option, value); err != nil {
			return fmt.Errorf(
				"could not set option `%s` on session `%s`: %w",
				option,
				a.session.Name,
				err,
			)
		}
	}

	return nil
}

// generateWindows iterates through the windows and panes defined within the
// specified profile and create them within the tmux session.
func (a *ActionUp) generateWindows(windows []*decoders.Window, first *tmux.Window) error {
	for i, ws := range windows {
		a.warnRename("window", ws.Name, tmux.SanitizeName(ws.Name))

		wtmx, err := a.createWindow(ws, i == 0, first)
		if err != nil {
			return err
		}

		if err := a.applyWindowOptions(ws, wtmx); err != nil {
			return err
		}

		defaultPane, err := a.getDefaultPane(wtmx)
		if err != nil {
			return err
		}

		if err := a.generatePanes(ws.Panes, defaultPane, wtmx); err != nil {
			return err
		}

		// Remove the default pane directly from the session.
		if defaultPane != nil {
			if err := defaultPane.Kill(); err != nil {
				return err
			}
		}

		for hook, command := range ws.Hooks {
			a.Logger.Info("setting window hook", "hook", hook, "window", wtmx.Name)
			if err := wtmx.SetHook(hook, command); err != nil {
				return fmt.Errorf(
					"could not set hook `%s` on window `%s`: %w",
					hook,
					wtmx.Name,
					err,
				)
			}
		}

		if err := wtmx.SelectLayout(ws.LayoutValue()); err != nil {
			return fmt.Errorf(
				"could not select layout `%s` for window `%s`: %w",
				ws.LayoutValue(),
				wtmx.Name,
				err,
			)
		}

		if ws.Focus {
			a.Logger.Info("setting window focus", "name", wtmx.Name)
			if err := wtmx.Select(); err != nil {
				a.Logger.Warn("could not focus window", "name", wtmx.Name, "error", err)
			}
		}
	}

	return nil
}

// applyWindowOptions sets the window options from the session block, then the window's own options, before any pane exists.
func (a *ActionUp) applyWindowOptions(ws *decoders.Window, wtmx *tmux.Window) error {
	for option, value := range a.windowDefaults {
		a.Logger.Info("setting session window option", "option", option, "window", wtmx.Name)
		if err := wtmx.SetOption(option, value); err != nil {
			return fmt.Errorf("could not set option `%s` on window `%s`: %w", option, wtmx.Name, err)
		}
	}

	for option, value := range ws.Options {
		if a.optionTables.IsSessionOnly(option) {
			if err := a.setSessionOption("window", wtmx.Name, option, value); err != nil {
				return err
			}

			continue
		}

		a.Logger.Info("setting window option", "option", option, "window", wtmx.Name)
		if err := wtmx.SetOption(option, value); err != nil {
			return fmt.Errorf("could not set option `%s` on window `%s`: %w", option, wtmx.Name, err)
		}
	}

	return nil
}

// setSessionOption warns that a session option declared on a window or pane applies to the whole session, then sets it.
func (a *ActionUp) setSessionOption(kind, name, option, value string) error {
	a.Logger.Warn(
		fmt.Sprintf("tmux keeps this option on the session, so it applies to the whole session and not only to this %s", kind),
		"option", option,
		kind, name,
	)

	if err := a.session.SetOption(option, value); err != nil {
		return fmt.Errorf("could not set option `%s` on session `%s`: %w", option, a.session.Name, err)
	}

	return nil
}

func (a *ActionUp) generatePanes(
	panes []*decoders.Pane,
	defaultPane *tmux.Pane,
	wtmx *tmux.Window,
) error {
	// Create every pane before any command runs, so a pane whose shell exits cannot break a later split.
	created := make([]*tmux.Pane, 0, len(panes))
	target := defaultPane.Target()
	for _, ps := range panes {
		a.warnRename("pane", ps.Name, tmux.SanitizeName(ps.Name))
		a.Logger.Info("splitting pane", "name", ps.Name, "from", target)
		ptmx, err := wtmx.Split(target, ps.Name, ps.StartingDirectory)
		if err != nil {
			return fmt.Errorf(
				"could not create pane `%s` in window `%s`: %w",
				ps.Name,
				wtmx.Name,
				err,
			)
		}

		// Each split halves its parent, so share out the space again before the next split.
		if err := wtmx.SelectLayout(enums.LayoutTiled.String()); err != nil {
			return fmt.Errorf("could not make room for the next pane in window `%s`: %w", wtmx.Name, err)
		}

		created = append(created, ptmx)
		target = ptmx.Target()
	}

	for i, ps := range panes {
		if err := a.configurePane(ps, created[i], wtmx); err != nil {
			return err
		}
	}

	return nil
}

// configurePane applies the hooks, options, commands, size, adjustments and focus of a pane.
func (a *ActionUp) configurePane(ps *decoders.Pane, ptmx *tmux.Pane, wtmx *tmux.Window) error {
	for hook, command := range ps.Hooks {
		a.Logger.Info("setting pane hook", "hook", hook, "pane", ptmx.Name)
		if err := ptmx.SetHook(hook, command); err != nil {
			return fmt.Errorf(
				"could not set hook `%s` on pane `%s` in window `%s`: %w",
				hook,
				ptmx.Name,
				wtmx.Name,
				err,
			)
		}
	}

	for option, value := range ps.Options {
		if a.optionTables.IsSessionOnly(option) {
			if err := a.setSessionOption("pane", ptmx.Name, option, value); err != nil {
				return err
			}

			continue
		}

		a.Logger.Info("setting pane option", "option", option, "pane", ptmx.Name)
		if err := ptmx.SetOption(option, value); err != nil {
			return fmt.Errorf(
				"could not set option `%s` on pane `%s` in window `%s`: %w",
				option,
				ptmx.Name,
				wtmx.Name,
				err,
			)
		}
	}

	if err := a.runCommands("pane", ptmx.Name, ptmx.Target(), ps.Commands); err != nil {
		return fmt.Errorf("could not run the commands for pane `%s` in window `%s`: %w", ptmx.Name, wtmx.Name, err)
	}

	if ps.Size.Valid() {
		a.Logger.Info("setting size", "x", ps.Size.X, "y", ps.Size.Y, "name", ptmx.Name)
		if err := ptmx.Resize(ps.Size.X, ps.Size.Y); err != nil {
			a.Logger.Warn("could not resize pane", "name", ptmx.Name, "error", err)
		}
	}

	// Apply any directional resize adjustments in the order they were
	// defined, after the absolute size so they refine the final dimensions.
	for _, adjustment := range ps.Adjustments {
		a.Logger.Info(
			"adjusting pane",
			"direction", adjustment.Direction,
			"amount", adjustment.Amount,
			"name", ptmx.Name,
		)

		if err := ptmx.Adjust(adjustment.Direction, adjustment.Amount); err != nil {
			return fmt.Errorf(
				"could not adjust pane `%s` in window `%s`: %w",
				ptmx.Name,
				wtmx.Name,
				err,
			)
		}
	}

	if ps.Focus {
		a.Logger.Info("setting pane focus", "name", ptmx.Name)
		if err := ptmx.Select(); err != nil {
			a.Logger.Warn("could not focus pane", "name", ptmx.Name, "error", err)
		}
	}

	return nil
}

// runCommands runs commands in the target pane. A shell that exits early or a timeout only warns, because the session is still usable.
func (a *ActionUp) runCommands(kind, name, target string, commands []string) error {
	if len(commands) == 0 {
		return nil
	}

	if a.runner == nil {
		runner, err := a.tmux.NewCommandRunner(a.commandTimeout)
		if err != nil {
			return err
		}

		a.runner = runner
	}

	for _, cmd := range commands {
		a.Logger.Info(fmt.Sprintf("setting %s command", kind), "cmd", cmd, "name", name)
	}

	err := a.runner.Run(target, commands)
	if errors.Is(err, tmux.ErrPaneExited) || errors.Is(err, tmux.ErrCommandTimeout) {
		a.Logger.Warn(fmt.Sprintf("glaze stopped waiting for the %s commands", kind), "name", name, "reason", err)
		return nil
	}

	return err
}

// getDefaultPane is responsible for retrieving the default pane for a given tmux window.
func (a *ActionUp) getDefaultPane(window *tmux.Window) (*tmux.Pane, error) {
	panes, err := a.tmux.Panes(window)
	if err != nil {
		return nil, fmt.Errorf("could not read panes for window `%s`: %w", window.Name, err)
	}

	if len(panes) == 0 {
		return nil, fmt.Errorf("could not locate default pane for window `%s`", window.Name)
	}

	// The pane that tmux creates with a window has the lowest id.
	return slices.MinFunc(panes, func(x, y *tmux.Pane) int {
		return int(x.Id) - int(y.Id)
	}), nil
}

// getFirstWindow returns the window that tmux creates with a new session, which has the lowest id.
func (a *ActionUp) getFirstWindow(session *tmux.Session) (*tmux.Window, error) {
	windows, err := a.tmux.Windows(session)
	if err != nil {
		return nil, fmt.Errorf("could not read windows for session `%s`: %w", session.Name, err)
	}

	if len(windows) == 0 {
		return nil, fmt.Errorf("could not find the first window of session `%s`", session.Name)
	}

	return slices.MinFunc(windows, func(x, y *tmux.Window) int {
		return int(x.Id) - int(y.Id)
	}), nil
}

// createWindow renames the first window of a new session for the first declared window, and creates the others.
func (a *ActionUp) createWindow(ws *decoders.Window, isFirst bool, first *tmux.Window) (*tmux.Window, error) {
	if isFirst && first != nil {
		a.Logger.Info("using the first window", "name", ws.Name)
		if err := first.Rename(ws.Name); err != nil {
			return nil, fmt.Errorf("could not rename the first window to `%s`: %w", ws.Name, err)
		}

		return first, nil
	}

	a.Logger.Info("creating new window", "name", ws.Name)
	wtmx, err := a.session.NewWindow(ws.Name, ws.StartingDirectory)
	if err != nil {
		return nil, fmt.Errorf("could not create new window `%s`: %w", ws.Name, err)
	}

	return wtmx, nil
}

// warnRename tells the user when glaze must change a name because tmux would rewrite it.
func (a *ActionUp) warnRename(kind, name, sanitized string) {
	if name != sanitized {
		a.Logger.Warn(
			fmt.Sprintf("tmux cannot use some characters in this %s name; replacing them with hyphens", kind),
			"name", name,
			"tmux_name", sanitized,
		)
	}
}

// resolveSession resolves the tmux session for this run. It returns true when
// the session already existed (in which case it has also attached to it, unless
// detached) and false when a brand new session was created and still needs to be
// provisioned by the caller.
func (a *ActionUp) resolveSession(profile *decoders.Session) (bool, error) {
	a.warnRename("session", profile.Name, tmux.SanitizeSessionName(profile.Name))

	if a.Command.Bool("clear") {
		current, err := a.tmux.CurrentSession()
		if err != nil {
			return false, fmt.Errorf("could not determine current session: %w", err)
		}

		if current != nil && current.Name == tmux.SanitizeSessionName(profile.Name) {
			return false, fmt.Errorf("glaze runs inside session `%s`, so --clear would also end glaze; run it from another session or outside tmux", current.Name)
		}

		a.Logger.Info("clearing previous session", "name", profile.Name)
		if err := a.tmux.KillSessionByName(profile.Name); err != nil {
			a.Logger.Warn("could not kill session", "name", profile.Name, "reason", err)
		}
	}

	exists, err := a.tmux.HasSession(profile.Name)
	if err != nil {
		return false, fmt.Errorf("could not check for session `%s`: %w", profile.Name, err)
	}

	if exists {
		return true, a.useExistingSession(profile)
	}

	a.Logger.Info("creating new session", "name", profile.Name)
	session, err := a.tmux.NewSession(profile.Name, profile.StartingDirectory)
	if errors.Is(err, tmux.ErrDuplicateSession) {
		// Another run created the session after has-session, so this run does not own it.
		a.Logger.Info("another run created the session first", "name", profile.Name)
		return true, a.useExistingSession(profile)
	}

	if err != nil {
		return false, fmt.Errorf("could not create new session `%s`: %w", profile.Name, err)
	}

	a.session = session

	return false, nil
}

// useExistingSession finds the running session for the profile and attaches to it, unless --detached is set.
func (a *ActionUp) useExistingSession(profile *decoders.Session) error {
	session, err := a.tmux.FindSessionByName(profile.Name)
	if err != nil {
		return fmt.Errorf("could not find session `%s`: %w", profile.Name, err)
	}

	a.session = session

	if !a.Command.Bool("detached") {
		a.Logger.Info("attaching to existing session", "name", profile.Name)
	}

	return a.attachToSession()
}
