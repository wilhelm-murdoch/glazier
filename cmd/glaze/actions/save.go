package actions

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/urfave/cli/v3"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// savedPane is an intermediate representation of a tmux pane captured for output.
type savedPane struct {
	Name              string
	StartingDirectory string
	Focus             bool
}

// savedWindow is an intermediate representation of a tmux window captured for output.
type savedWindow struct {
	Name   string
	Layout string
	Focus  bool
	Panes  []savedPane
}

// savedSession is an intermediate representation of a tmux session captured for
// output. It deliberately decouples HCL generation from the live tmux types so
// the generator can be unit tested without a tmux server.
type savedSession struct {
	Name              string
	StartingDirectory string
	Windows           []savedWindow
}

// ActionSave is a struct that represents a Glazier "action".
type ActionSave struct {
	Command *cli.Command
	Logger  *logger.Logger
	tmux    *tmux.Client
}

// NewSave is responsible for creating a new ActionSave struct value. Unlike the
// other actions, save writes a profile rather than reading one, so it does not
// resolve or parse an existing profile file.
func NewSave(cmd *cli.Command, logLevel string) (*ActionSave, error) {
	log := logger.New(logger.FriendlyToInternal[logLevel])

	tmuxClient, err := tmux.NewClient(
		cmd.String("socket-path"),
		cmd.String("socket-name"),
		log.Logger,
	)
	if err != nil {
		return nil, err
	}

	return &ActionSave{
		Command: cmd,
		Logger:  log,
		tmux:    tmuxClient,
	}, nil
}

// Run captures the state of a running tmux session and writes it to a glaze
// profile, either on disk or to stdout.
func (a *ActionSave) Run() error {
	if !a.tmux.IsRunning() {
		return fmt.Errorf("no running tmux server found")
	}

	a.Logger.Warn("this feature is currently EXPERIMENTAL and is limited to exporting structural layouts ONLY")

	path := a.Command.String("profile-path")
	if path == "" {
		path = ".glaze"
	}

	session, err := a.resolveSession()
	if err != nil {
		return err
	}

	a.Logger.Info("saving session", "session", session.Name, "path", path)

	captured, err := a.captureSession(session)
	if err != nil {
		return err
	}

	output := generateProfile(captured)

	if a.Command.Bool("stdout") {
		fmt.Print(string(output))
		return nil
	}

	// Profiles are sharable config meant to be committed; 0644 is intended.
	if err := os.WriteFile(path, output, 0o644); err != nil { //nolint:gosec // G306
		return fmt.Errorf("could not write profile `%s`: %w", path, err)
	}

	a.Logger.Info("saved session", "session", captured.Name, "path", path)

	return nil
}

// resolveSession determines which tmux session to capture, preferring the
// --session flag and falling back to the session of the current client.
func (a *ActionSave) resolveSession() (*tmux.Session, error) {
	name := a.Command.String("session")
	if name == "" {
		current, err := a.tmux.CurrentSessionName()
		if err != nil {
			return nil, err
		}
		name = current
	}

	session, err := a.tmux.FindSessionByName(name)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// captureSession reads the windows and panes of the given session into the
// intermediate model used for HCL generation.
func (a *ActionSave) captureSession(session *tmux.Session) (savedSession, error) {
	captured := savedSession{
		Name:              session.Name,
		StartingDirectory: session.StartingDirectory,
	}

	windows, err := a.tmux.Windows(session)
	if err != nil {
		return captured, err
	}

	for _, window := range windows {
		a.Logger.Info("capturing window", "name", window.Name)

		sw := savedWindow{
			Name: window.Name,

			// tmux reports a window's layout as a coordinate string (e.g.
			// "bb62,80x24,0,0"), not one of glaze's named presets, so a preset
			// cannot be faithfully recovered. We capture the raw coordinate
			// string verbatim as a fallback: `up` replays it via select-layout
			// (which accepts a raw string), and glaze's layout validation
			// accepts a well-formed coordinate string in addition to the named
			// presets. This is a structural snapshot of geometry, not a preset
			// guess - see the "raw layout" note in AGENTS.md.
			Layout: window.RawLayout,

			// The active window is the one tmux would focus on attach.
			Focus: window.IsActive,
		}

		panes, err := a.tmux.Panes(window)
		if err != nil {
			return captured, err
		}

		for _, pane := range panes {
			a.Logger.Info("capturing pane", "name", pane.Name)
			sw.Panes = append(sw.Panes, savedPane{
				Name:              pane.Name,
				StartingDirectory: pane.StartingDirectory,

				// The active pane is the one tmux would focus within the window.
				Focus: pane.IsActive,
			})
		}

		captured.Windows = append(captured.Windows, sw)
	}

	return captured, nil
}

// generateProfile renders the captured session as a formatted glaze (HCL)
// definition file.
//
// Note: save deliberately emits a structural snapshot only (names, layout,
// starting directories). It does NOT export commands, envs, hooks, or
// options: tmux introspection returns effective state (leaking secrets and
// out-of-band config), and commands/hooks are arbitrary code that would
// re-execute on the next `up`. Do NOT add those fields here.
func generateProfile(session savedSession) []byte {
	file := hclwrite.NewEmptyFile()

	sessionBlock := file.Body().AppendNewBlock("session", nil)
	sessionBody := sessionBlock.Body()
	sessionBody.SetAttributeValue("name", cty.StringVal(session.Name))
	if session.StartingDirectory != "" {
		sessionBody.SetAttributeValue("starting_directory", cty.StringVal(session.StartingDirectory))
	}

	for _, window := range session.Windows {
		sessionBody.AppendNewline()

		windowBlock := sessionBody.AppendNewBlock("window", nil)
		windowBody := windowBlock.Body()
		windowBody.SetAttributeValue("name", cty.StringVal(window.Name))
		if window.Layout != "" {
			windowBody.SetAttributeValue("layout", cty.StringVal(window.Layout))
		}
		if window.Focus {
			windowBody.SetAttributeValue("focus", cty.BoolVal(true))
		}

		for _, pane := range window.Panes {
			windowBody.AppendNewline()

			paneBlock := windowBody.AppendNewBlock("pane", nil)
			paneBody := paneBlock.Body()
			paneBody.SetAttributeValue("name", cty.StringVal(pane.Name))
			if pane.StartingDirectory != "" {
				paneBody.SetAttributeValue("starting_directory", cty.StringVal(pane.StartingDirectory))
			}
			if pane.Focus {
				paneBody.SetAttributeValue("focus", cty.BoolVal(true))
			}
		}
	}

	return hclwrite.Format(file.Bytes())
}
