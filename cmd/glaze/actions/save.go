package actions

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/urfave/cli/v3"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
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

// savedSession is the captured session, kept apart from the tmux types so the generator can be tested without tmux.
type savedSession struct {
	Name              string
	StartingDirectory string
	Windows           []savedWindow
}

// ActionSave captures a running session and writes it as a profile.
type ActionSave struct {
	Command *cli.Command
	Logger  *logger.Logger
	tmux    tmux.Client
}

// NewSave returns the save action, which writes a profile and so reads none.
func NewSave(cmd *cli.Command, logLevel string) (*ActionSave, error) {
	log := newLogger(cmd, logLevel)

	tmuxClient, err := newTmuxClient(cmd, log)
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
func (a *ActionSave) Run(ctx context.Context) error {
	a.tmux = a.tmux.WithContext(ctx)

	running, err := a.tmux.IsRunning()
	if err != nil {
		return err
	}

	if !running {
		return errors.New("no tmux server is running, so there is no session to save")
	}

	a.Logger.Warn("this feature is currently EXPERIMENTAL and is limited to exporting structural layouts ONLY")

	path := a.Command.String("profile-path")
	if path == "" {
		path = ".glaze"
	}

	// Check before the capture, so that a refusal captures nothing. A dangling symlink also counts as a file.
	if _, err := os.Lstat(path); err == nil && !a.Command.Bool("stdout") && !a.Command.Bool("force") {
		return fmt.Errorf("could not save to `%s`: %w; use --force to replace it", path, files.ErrFileExists)
	}

	session, err := a.resolveSession()
	if err != nil {
		return err
	}

	if a.Command.Bool("stdout") {
		a.Logger.Info("saving session", "session", session.Name, "output", "stdout")
	} else {
		a.Logger.Info("saving session", "session", session.Name, "path", path)
	}

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
	if err := files.WriteFile(path, output, 0o644); err != nil {
		return fmt.Errorf("could not write profile `%s`: %w", path, err)
	}

	a.Logger.Info("saved session", "session", captured.Name, "path", path)

	return nil
}

// resolveSession returns the session named by --session, or else the session of the pane that glaze runs in.
func (a *ActionSave) resolveSession() (*tmux.Session, error) {
	name := a.Command.String("session")
	if name == "" {
		current, err := a.tmux.CurrentSession()
		if err != nil {
			return nil, err
		}

		// Outside a pane of this server, tmux would pick an arbitrary session.
		if current == nil {
			return nil, errors.New("glaze does not run inside this tmux server, so give the session to save with --session")
		}

		return current, nil
	}

	return a.tmux.FindSessionByName(name)
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

			// tmux reports a layout only as a coordinate string, so save keeps the string as it is.
			// `up` replays it with select-layout, and validation accepts a well-formed layout string.
			Layout: window.RawLayout,
			Focus:  window.IsActive, // The active window is the one tmux would focus on attach.
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
				Focus:             pane.IsActive, // The active pane is the one tmux would focus within the window.
			})
		}

		captured.Windows = append(captured.Windows, sw)
	}

	return captured, nil
}

// generateProfile renders the captured session as a formatted profile, with names, directories, focus and layout only.
// Do not add commands, envs, hooks or options: tmux reports effective state that can hold secrets, and commands run again on `up`.
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
