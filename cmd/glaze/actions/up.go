package actions

import (
	"fmt"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/spec"
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
func NewUp(cmd *cli.Command, log *logger.Logger) (*ActionUp, error) {
	base, err := NewActionBase(cmd, log)
	if err != nil {
		return nil, err
	}

	tmuxClient := tmux.NewClient(
		cmd.String("socket-name"),
		cmd.String("socket-path"),
		log.Logger,
	)

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

	isAlreadyAttached, err := a.resolveSession(profile)
	if err != nil {
		return err
	}

	if err := a.provisionSession(profile); err != nil {
		return fmt.Errorf("failed to provision session: %w", err)
	}

	// We're attached to a pre-existing session, so there is no need to do anything else here:
	if isAlreadyAttached {
		return nil
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
	if err := a.generateWindows(profile.Windows.Items()); err != nil {
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

	return nil
}

// loadProfile handles parsing variables and decoding the HCL definition.
func (a *ActionUp) loadProfile() (*decoders.Session, error) {
	variables, err := parser.CollectVariables(a.Command.StringSlice("var"))
	if err != nil {
		return nil, fmt.Errorf("could not parse specified variables: %w", err)
	}

	profile, decodeDiags := a.Parser.Decode(
		spec.Session,
		parser.BuildEvalContext(variables),
	)
	if decodeDiags.HasErrors() {
		a.DiagnosticsManager.Extend(decodeDiags)
		return nil, a.DiagnosticsManager.Write()
	}

	return profile, nil
}

// generateWindows iterates through the windows and panes defined within the
// specified profile and create them within the tmux session.
func (a *ActionUp) generateWindows(windows []*decoders.Window) error {
	for _, ws := range windows {
		a.Logger.Info("creating new window", "name", ws.Name)
		wtmx, err := a.session.NewWindow(ws.Name)
		if err != nil {
			return fmt.Errorf("could not create new window `%s`: %w", ws.Name, err)
		}

		defaultPane, err := a.getDefaultPane(wtmx)
		if err != nil {
			return err
		}

		// Panes are originally parsed and created in the reverse order of how they
		// are defined within the glaze definition file. So, we'll just reverse them
		// here to set them back to the user-defined order. The reasoning is that a
		// user will expect these panes to be arranged in the same order as they were
		// defined within the definition file.
		if err := a.generatePanes(ws.Panes.Reverse().Items(), defaultPane, wtmx); err != nil {
			return err
		}

		// Remove the default pane directly from the session.
		if defaultPane != nil {
			if err := defaultPane.Kill(); err != nil {
				return err
			}
		}

		if err := wtmx.SelectLayout(ws.Layout); err != nil {
			return fmt.Errorf(
				"could not select layout `%s` for window `%s`: %w",
				ws.Layout,
				wtmx.Name,
				err,
			)
		}

		if ws.Focus {
			a.Logger.Info("setting window focus", "name", wtmx.Name)
			wtmx.Select()
		}
	}

	return nil
}

func (a *ActionUp) generatePanes(
	panes []*decoders.Pane,
	defaultPane *tmux.Pane,
	wtmx *tmux.Window,
) error {
	for _, ps := range panes {
		a.Logger.Info("splitting pane", "name", ps.Name, "from", defaultPane.Target())
		ptmx, err := wtmx.Split(defaultPane.Target(), ps.Name, ps.StartingDirectory)
		if err != nil {
			return fmt.Errorf(
				"could not split pane `%d` for window `%s`: %w",
				defaultPane.Index,
				wtmx.Name,
				err,
			)
		}

		// Run any defined commands in order as defined within the current profile. Add a small delay between each command to ensure they are executed in order.
		for _, cmd := range ps.Commands {
			a.Logger.Info("setting pane command", "cmd", cmd, "name", ptmx.Name)
			time.Sleep(time.Millisecond * time.Duration(100))
			if err := ptmx.SendKeys(cmd); err != nil {
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
			ptmx.Resize(ps.Size.X, ps.Size.Y)
		}

		if ps.Focus {
			a.Logger.Info("setting pane focus", "name", ptmx.Name)
			ptmx.Select()
		}
	}

	return nil
}

// getDefaultPane is responsible for retrieving the default pane for a given tmux window.
func (a *ActionUp) getDefaultPane(window *tmux.Window) (*tmux.Pane, error) {
	panes, err := a.tmux.Panes(window)
	if err != nil {
		return nil, fmt.Errorf("could not read panes for window `%s`: %w", window.Name, err)
	}

	defaultPane := panes.Find(func(i int, item *tmux.Pane) bool {
		return item.IsFirst
	})

	if defaultPane == nil {
		return nil, fmt.Errorf("could not locate default pane for window `%s`", window.Name)
	}

	return defaultPane, nil
}

// getDefaultWindow is responsible for retrieving the default window for a given tmux session.
func (a *ActionUp) getDefaultWindow(session *tmux.Session) (*tmux.Window, error) {
	windows, err := a.tmux.Windows(session)
	if err != nil {
		return nil, fmt.Errorf("could not read windows for session `%s`: %w", session.Name, err)
	}

	defaultWindow := windows.Find(func(i int, window *tmux.Window) bool {
		return window.IsFirst
	})

	if defaultWindow == nil {
		return nil, fmt.Errorf("could not locate default window for session `%s`", session.Name)
	}

	return defaultWindow, nil
}

// resolveSession is responsible for resolving the tmux session, either by attaching to an existing one or creating a new one.
func (a *ActionUp) resolveSession(profile *decoders.Session) (bool, error) {
	attached := false

	if a.Command.Bool("clear") {
		a.Logger.Info("clearing previous session", "name", profile.Name)
		if err := a.tmux.KillSessionByName(profile.Name); err != nil {
			a.Logger.Warn("could not kill session", "name", profile.Name, "reason", err)
		}
	}

	if a.tmux.HasSession(profile.Name) {
		session, err := a.tmux.FindSessionByName(profile.Name)
		if err != nil {
			return attached, fmt.Errorf("could not find session `%s`: %w", profile.Name, err)
		}

		if !a.Command.Bool("detached") {
			a.Logger.Info("attaching to existing session", "name", profile.Name)
			if err := a.tmux.Attach(session); err != nil {
				return attached, fmt.Errorf(
					"could not attach to session `%s`: %w",
					session.Name,
					err,
				)
			}

			attached = true
		}

		return attached, nil
	}

	a.Logger.Info("creating new session", "name", profile.Name)
	session, err := a.tmux.NewSession(profile.Name, profile.StartingDirectory)
	if err != nil {
		return attached, fmt.Errorf("could not create new session `%s`: %w", session.Name, err)
	}

	a.session = session

	return attached, nil
}
