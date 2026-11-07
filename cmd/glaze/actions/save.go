package actions

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/urfave/cli/v3"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

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

	if err := a.tmux.CheckVersion(); err != nil {
		return err
	}

	running, err := a.tmux.IsRunning()
	if err != nil {
		return err
	}

	if !running {
		return errors.New("no tmux server is running, so there is no session to save")
	}

	path, inRegistry, err := saveTarget(a.Command)
	if err != nil {
		return err
	}

	if inRegistry {
		warnIgnoredConfigHome(a.Logger)
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

	captured, err := captureSession(a.tmux, session, a.Logger)
	if err != nil {
		return err
	}

	pruneMissingDirectories(&captured, a.Logger)

	output := generateProfile(captured)

	if a.Command.Bool("stdout") {
		fmt.Print(string(output))
		return nil
	}

	if inRegistry {
		if err := createRegistry(filepath.Dir(path)); err != nil {
			return err
		}
	}

	// Profiles are sharable config meant to be committed; 0644 is intended.
	if err := files.WriteFile(path, output, 0o644); err != nil {
		return fmt.Errorf("could not write profile `%s`: %w", path, err)
	}

	a.Logger.Info("saved session", "session", captured.Name, "path", path)

	return nil
}

// saveTarget returns the file to write: the registry profile of a positional name, or else --profile-path or `.glaze`.
func saveTarget(cmd *cli.Command) (path string, inRegistry bool, err error) {
	if name := profileName(cmd); name != "" {
		path, err := files.RegistryProfilePath(name)
		return path, true, err
	}

	path = cmd.String("profile-path")
	if path == "" {
		path = ".glaze"
	}

	return path, false, nil
}

// createRegistry makes the registry directory when it is missing; a registry write is the only thing that creates it.
// The mode is 0700, the XDG rule for a directory that a program creates, because a profile can hold a secret in envs.
func createRegistry(dir string) error {
	err := files.CheckRegistryDir(dir)
	if err == nil {
		return nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("could not create the profile registry `%s`: %w", dir, err)
	}

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
		if window.Name != "" {
			windowBody.SetAttributeValue("name", cty.StringVal(window.Name))
		}

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
			if pane.Name != "" {
				paneBody.SetAttributeValue("name", cty.StringVal(pane.Name))
			}

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
