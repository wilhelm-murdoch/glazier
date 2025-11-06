package up

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v2"

	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/schema"
	"github.com/wilhelm-murdoch/glazier/internal/schema/pane"
	"github.com/wilhelm-murdoch/glazier/internal/schema/session"
	"github.com/wilhelm-murdoch/glazier/internal/schema/window"
	"github.com/wilhelm-murdoch/glazier/internal/tmux"
)

type Action struct {
	actions.BaseAction
	client  *tmux.Client
	session *tmux.Session
}

// NewAction is responsible for creating a new Action instance.
func NewAction(ctx *cli.Context) (*Action, error) {
	base, err := actions.NewBaseAction(ctx)
	if err != nil {
		return nil, err
	}

	client := tmux.NewClient(
		ctx.String("socket-name"),
		ctx.String("socket-path"),
		ctx.Bool("debug"),
	)

	return &Action{
		BaseAction: *base,
		client:     client,
	}, nil
}

// Run is responsible for executing the up action, which includes parsing variables, decoding the profile, resolving the session, and generating windows.
func (a *Action) Run() error {
	variables, err := parser.CollectVariables(a.Context.StringSlice("var"))
	if err != nil {
		return fmt.Errorf("could not parse specified variables: %w", err)
	}

	profile, decodeDiags := a.Parser.Decode(
		schema.PrimaryGlazeSpec,
		parser.BuildEvalContext(variables),
	)

	if decodeDiags.HasErrors() {
		a.DiagnosticsManager.Extend(decodeDiags)
		return a.DiagnosticsManager.Write()
	}

	attached, err := a.resolveSession(profile)
	if err != nil {
		return err
	}

	// We're attached to a pre-existing session, so there is no need to do anything else here:
	if attached {
		return nil
	}

	if err := a.generateWindows(profile.Windows.Items()); err != nil {
		return err
	}

	defaultWindow, err := a.getDefaultWindow(a.session)
	if err != nil {
		return err
	}

	if defaultWindow != nil {
		if err := defaultWindow.Kill(); err != nil {
			return err
		}
	}

	if !a.Context.Bool("detached") {
		if err := a.client.Attach(a.session); err != nil {
			return err
		}
	}

	return nil
}

// generateWindows iterates through the windows and panes defined within the specified profile and create them within the tmux session.
func (a *Action) generateWindows(windows []*window.Window) error {
	for _, ws := range windows {
		log.Info("creating new window", "window", ws.Name)
		wtmx, err := a.session.NewWindow(ws.Name)
		if err != nil {
			return fmt.Errorf("could not create new window `%s`: %w", ws.Name, err)
		}

		defaultPane, err := a.getDefaultPane(wtmx)
		if err != nil {
			return err
		}

		// Panes are originally parsed and created in the reverse order of how they are defined within the glaze definition file. So, we'll just reverse them here to set them back to the user-defined order.
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
			log.Info("setting focus", "window", wtmx.Name)
			wtmx.Select()
		}
	}

	return nil
}

func (a *Action) generatePanes(
	panes []*pane.Pane,
	defaultPane *tmux.Pane,
	wtmx *tmux.Window,
) error {
	for _, ps := range panes {
		log.Info("adding", "pane", ps.Name, "from", defaultPane.Target())
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
			log.Info("setting", "command", cmd, "pane", ptmx.Name)
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
			log.Debug("setting size", "x", ps.Size.X, "y", ps.Size.Y, "pane", ptmx.Name)
			ptmx.Resize(ps.Size)
		}

		if ps.Focus {
			log.Info("setting focus", "pane", ptmx.Name)
			ptmx.Select()
		}
	}

	return nil
}

// getDefaultPane is responsible for retrieving the default pane for a given tmux window.
func (a *Action) getDefaultPane(window *tmux.Window) (*tmux.Pane, error) {
	panes, err := a.client.Panes(window)
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
func (a *Action) getDefaultWindow(session *tmux.Session) (*tmux.Window, error) {
	windows, err := a.client.Windows(session)
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
func (a *Action) resolveSession(profile *session.Session) (bool, error) {
	attached := false

	if a.Context.Bool("clear") {
		log.Info("clearing previous session", "session", profile.Name)
		if err := a.client.KillSessionByName(profile.Name); err != nil {
			log.Debug("could not kill session", "session", profile.Name, "reason", err)
		}
	}

	if a.client.SessionExists(profile.Name) {
		session, err := a.client.FindSessionByName(profile.Name)
		if err != nil {
			return attached, fmt.Errorf("could not find session `%s`: %w", profile.Name, err)
		}

		if !a.Context.Bool("detached") {
			log.Info("attaching to existing session", "session", profile.Name)
			if err := a.client.Attach(session); err != nil {
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

	log.Info("creating new session", "session", profile.Name)
	session, err := a.client.NewSession(profile.Name, profile.StartingDirectory)
	if err != nil {
		return attached, fmt.Errorf("could not create new session `%s`: %w", session.Name, err)
	}

	a.session = session

	return attached, nil
}
