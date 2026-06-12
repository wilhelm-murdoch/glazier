package actions

import (
	"fmt"
	"slices"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// ActionUp is a struct that represents a Glazier "action".
type ActionUp struct {
	ActionBase
	tmux    *tmux.Client
	session *tmux.Session
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
		ActionBase: *base,
		tmux:       tmuxClient,
	}, nil
}

// Run is responsible for executing the up action, which includes parsing variables,
// decoding the profile, resolving the session, and generating windows.
func (a *ActionUp) Run() error {
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
		return fmt.Errorf("failed to provision session: %w", err)
	}

	return a.attachToSession()
}

// attachToSession handles attaching the tmux client to the newly created session.
func (a *ActionUp) attachToSession() error {
	if !a.Command.Bool("detached") {
		if err := a.tmux.Attach(a.session); err != nil {
			return err
		}
	}

	return nil
}

// provisionSession creates the windows and panes as defined in the profile.
func (a *ActionUp) provisionSession(profile *decoders.Session) error {
	if err := a.applySessionSettings(profile); err != nil {
		return err
	}

	if err := a.generateWindows(profile.Windows); err != nil {
		return err
	}

	defaultWindow, err := a.getDefaultWindow(a.session)
	if err != nil {
		a.Logger.Warn(
			"could not find default window to kill",
			"session",
			a.session.Name,
			"error",
			err,
		)
	} else if defaultWindow != nil {
		// After creating our own windows, we can remove the default one tmux created.
		if err := defaultWindow.Kill(); err != nil {
			return fmt.Errorf("failed to kill default window: %w", err)
		}
	}

	// Run any session-level commands in the session's active pane, once all
	// windows and panes exist. Each is serialised via `tmux wait-for`, except
	// the final command which is sent fire-and-forget so a long-running or
	// interactive command does not block on a wait-for signal that never fires.
	for i, cmd := range profile.Commands {
		a.Logger.Info("setting session command", "cmd", cmd, "session", a.session.Name)
		if i == len(profile.Commands)-1 {
			if err := a.session.SendKeys(cmd); err != nil {
				return fmt.Errorf(
					"could not execute command `%s` for session `%s`: %w",
					cmd,
					a.session.Name,
					err,
				)
			}
			continue
		}
		channel := fmt.Sprintf("glaze-session-%s-%d", a.session.Name, i)
		if err := a.session.SendKeysAndWait(cmd, channel); err != nil {
			return fmt.Errorf(
				"could not execute command `%s` for session `%s`: %w",
				cmd,
				a.session.Name,
				err,
			)
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
func (a *ActionUp) generateWindows(windows []*decoders.Window) error {
	for _, ws := range windows {
		a.Logger.Info("creating new window", "name", ws.Name)
		wtmx, err := a.session.NewWindow(ws.Name, ws.StartingDirectory)
		if err != nil {
			return fmt.Errorf("could not create new window `%s`: %w", ws.Name, err)
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

		for key, value := range ws.Envs {
			a.Logger.Info("setting window env", "key", key, "window", wtmx.Name)
			if err := wtmx.SetEnv(key, value); err != nil {
				return fmt.Errorf(
					"could not set env `%s` on window `%s`: %w",
					key,
					wtmx.Name,
					err,
				)
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

		for option, value := range ws.Options {
			a.Logger.Info("setting window option", "option", option, "window", wtmx.Name)
			if err := wtmx.SetOption(option, value); err != nil {
				return fmt.Errorf(
					"could not set option `%s` on window `%s`: %w",
					option,
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

func (a *ActionUp) generatePanes(
	panes []*decoders.Pane,
	defaultPane *tmux.Pane,
	wtmx *tmux.Window,
) error {
	target := defaultPane.Target()
	for _, ps := range panes {
		a.Logger.Info("splitting pane", "name", ps.Name, "from", defaultPane.Target())
		ptmx, err := wtmx.Split(target, ps.Name, ps.StartingDirectory)
		if err != nil {
			return fmt.Errorf(
				"could not split pane `%d` for window `%s`: %w",
				defaultPane.Index,
				wtmx.Name,
				err,
			)
		}

		// Apply any pane-scoped environment variables and hooks before running
		// commands so they are in effect for the pane's shell.
		for key, value := range ps.Envs {
			a.Logger.Info("setting pane env", "key", key, "pane", ptmx.Name)
			if err := ptmx.SetEnv(key, value); err != nil {
				return fmt.Errorf(
					"could not set env `%s` on pane `%s` in window `%s`: %w",
					key,
					ptmx.Name,
					wtmx.Name,
					err,
				)
			}
		}

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

		// Run any defined commands in order as defined within the current
		// profile. Each command is serialised using `tmux wait-for` so the
		// next command is only sent once the previous one has completed. The
		// final command has no successor to gate, so it is sent
		// fire-and-forget: waiting on it would hang forever for long-running
		// or interactive commands (nvim, tail -f, a dev server) whose
		// wait-for signal never fires.
		for i, cmd := range ps.Commands {
			a.Logger.Info("setting pane command", "cmd", cmd, "name", ptmx.Name)
			if i == len(ps.Commands)-1 {
				if err := ptmx.SendKeys(cmd); err != nil {
					return fmt.Errorf(
						"could not execute command `%s` for pane `%s` in window `%s`: %w",
						cmd,
						ptmx.Name,
						wtmx.Name,
						err,
					)
				}
				continue
			}
			channel := fmt.Sprintf("glaze-%d-%d", int(ptmx.Id), i)
			if err := ptmx.SendKeysAndWait(cmd, channel); err != nil {
				return fmt.Errorf(
					"could not execute command `%s` for pane `%s` in window `%s`: %w",
					cmd,
					ptmx.Name,
					wtmx.Name,
					err,
				)
			}
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

		target = ptmx.Target()
	}

	return nil
}

// getDefaultPane is responsible for retrieving the default pane for a given tmux window.
func (a *ActionUp) getDefaultPane(window *tmux.Window) (*tmux.Pane, error) {
	panes, err := a.tmux.Panes(window)
	if err != nil {
		return nil, fmt.Errorf("could not read panes for window `%s`: %w", window.Name, err)
	}

	index := slices.IndexFunc(panes, func(pane *tmux.Pane) bool {
		return pane.IsFirst
	})

	if index == -1 {
		return nil, fmt.Errorf("could not locate default pane for window `%s`", window.Name)
	}

	return panes[index], nil
}

// getDefaultWindow is responsible for retrieving the default window for a given tmux session.
func (a *ActionUp) getDefaultWindow(session *tmux.Session) (*tmux.Window, error) {
	windows, err := a.tmux.Windows(session)
	if err != nil {
		return nil, fmt.Errorf("could not read windows for session `%s`: %w", session.Name, err)
	}

	index := slices.IndexFunc(windows, func(window *tmux.Window) bool {
		return window.IsFirst
	})

	if index == -1 {
		return nil, fmt.Errorf("could not locate default window for session `%s`", session.Name)
	}

	return windows[index], nil
}

// resolveSession resolves the tmux session for this run. It returns true when
// the session already existed (in which case it has also attached to it, unless
// detached) and false when a brand new session was created and still needs to be
// provisioned by the caller.
func (a *ActionUp) resolveSession(profile *decoders.Session) (bool, error) {
	if a.Command.Bool("clear") {
		a.Logger.Info("clearing previous session", "name", profile.Name)
		if err := a.tmux.KillSessionByName(profile.Name); err != nil {
			a.Logger.Warn("could not kill session", "name", profile.Name, "reason", err)
		}
	}

	if a.tmux.HasSession(profile.Name) {
		session, err := a.tmux.FindSessionByName(profile.Name)
		if err != nil {
			return true, fmt.Errorf("could not find session `%s`: %w", profile.Name, err)
		}

		a.session = session

		if !a.Command.Bool("detached") {
			a.Logger.Info("attaching to existing session", "name", profile.Name)
			if err := a.tmux.Attach(session); err != nil {
				return true, fmt.Errorf(
					"could not attach to session `%s`: %w",
					session.Name,
					err,
				)
			}
		}

		return true, nil
	}

	a.Logger.Info("creating new session", "name", profile.Name)
	session, err := a.tmux.NewSession(profile.Name, profile.StartingDirectory)
	if err != nil {
		return false, fmt.Errorf("could not create new session `%s`: %w", profile.Name, err)
	}

	a.session = session

	return false, nil
}
