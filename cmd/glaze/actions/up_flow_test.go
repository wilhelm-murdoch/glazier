package actions

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

const validProfile = `session {
  name = "demo"

  window {
    name   = "main"
    layout = "tiled"

    pane {
      name = "shell"
    }
  }
}
`

// buildUp builds ActionUp from a profile and up flags as main does, with a recorder for tmux.
// The real parser, diagnostics manager and tmux client run, but no tmux server.
func buildUp(t *testing.T, profile string, flags map[string]string) (*ActionUp, *tmuxtest.Recorder) {
	t.Helper()

	rec := tmuxtest.New().Default(tmuxtest.Result{})
	rec.Install(t)

	dir := t.TempDir()
	path := filepath.Join(dir, ".glaze")
	assert.NoError(t, os.WriteFile(path, []byte(profile), 0o600))

	var (
		action *ActionUp
		actErr error
	)

	cmd := &cli.Command{
		Name: "up",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "detached"},
			&cli.BoolFlag{Name: "clear"},
			&cli.BoolFlag{Name: "debug"},
			&cli.BoolFlag{Name: "keep-on-failure"},
			&cli.StringFlag{Name: "socket-path"},
			&cli.StringFlag{Name: "socket-name"},
			&cli.StringFlag{Name: "profile-path"},
			&cli.StringSliceFlag{Name: "var"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			action, actErr = NewUp(c, "critical")
			return nil
		},
	}

	args := []string{"up", "--profile-path", path}
	for k, v := range flags {
		args = append(args, "--"+k, v)
	}

	assert.NoError(t, cmd.Run(context.Background(), args))
	if actErr != nil {
		t.Skipf("could not construct ActionUp (tmux likely unavailable): %v", actErr)
	}

	return action, rec
}

func TestActionUpDebugFlag(t *testing.T) {
	build := func(t *testing.T, args []string) *ActionUp {
		t.Helper()

		rec := tmuxtest.New().Default(tmuxtest.Result{})
		rec.Install(t)

		dir := t.TempDir()
		path := filepath.Join(dir, ".glaze")
		assert.NoError(t, os.WriteFile(path, []byte(validProfile), 0o600))

		var (
			action *ActionUp
			actErr error
		)

		cmd := &cli.Command{
			Name: "up",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "debug"},
				&cli.StringFlag{Name: "profile-path"},
				&cli.StringSliceFlag{Name: "var"},
			},
			Action: func(_ context.Context, c *cli.Command) error {
				action, actErr = NewUp(c, "critical")
				return nil
			},
		}

		assert.NoError(t, cmd.Run(context.Background(), append([]string{"up", "--profile-path", path}, args...)))
		if actErr != nil {
			t.Skipf("could not construct ActionUp (tmux likely unavailable): %v", actErr)
		}

		return action
	}

	t.Run("forces the logger to debug level when set", func(t *testing.T) {
		up := build(t, []string{"--debug"})
		assert.Equal(t, logger.LevelDebug, up.Logger.Level)
	})

	t.Run("leaves the configured level untouched when unset", func(t *testing.T) {
		up := build(t, nil)
		assert.Equal(t, logger.LevelCritical, up.Logger.Level)
	})
}

func TestActionUpLoadProfile(t *testing.T) {
	t.Run("decodes a valid profile", func(t *testing.T) {
		up, _ := buildUp(t, validProfile, nil)

		profile, err := up.loadProfile()
		assert.NoError(t, err)
		assert.NotNil(t, profile)
		assert.Equal(t, "demo", profile.Name)
	})

	t.Run("resolves relative directories against the profile and fills in inherited ones", func(t *testing.T) {
		withDirs := `session {
  name               = "demo"
  starting_directory = "app"

  window {
    name = "main"
    pane { name = "a" }
    pane {
      name               = "b"
      starting_directory = "app/logs"
    }
  }
}
`
		up, _ := buildUp(t, withDirs, nil)
		dir := filepath.Dir(up.ProfilePath)
		assert.NoError(t, os.MkdirAll(filepath.Join(dir, "app", "logs"), 0o700))

		// The profile directory, not the current one, is the base of a relative path.
		t.Chdir(t.TempDir())

		profile, err := up.loadProfile()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "app"), profile.StartingDirectory)
		assert.Equal(t, filepath.Join(dir, "app"), profile.Windows[0].StartingDirectory)
		assert.Equal(t, filepath.Join(dir, "app"), profile.Windows[0].Panes[0].StartingDirectory)
		assert.Equal(t, filepath.Join(dir, "app", "logs"), profile.Windows[0].Panes[1].StartingDirectory)
	})

	t.Run("returns the diagnostics sentinel for an invalid layout", func(t *testing.T) {
		bad := `session {
  name = "demo"
  window {
    name   = "main"
    layout = "noop"
    pane { name = "shell" }
  }
}
`
		up, _ := buildUp(t, bad, nil)

		profile, err := up.loadProfile()
		assert.Nil(t, profile)
		assert.ErrorIs(t, err, diagnostics.ErrHasDiagnostics)
	})

	t.Run("ignores a malformed --var flag value without panicking", func(t *testing.T) {
		up, _ := buildUp(t, validProfile, nil)
		assert.NoError(t, up.Command.Set("var", "noequals"))

		assert.NotPanics(t, func() {
			profile, err := up.loadProfile()
			assert.NoError(t, err)
			assert.NotNil(t, profile)
		})
	})
}

func TestActionUpResolveSession(t *testing.T) {
	t.Run("creates a new session when none exists", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))
		rec.On("new", tmuxtest.Result{Output: "$1;demo;/tmp"})

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		existed, err := up.resolveSession(profile)
		assert.NoError(t, err)
		assert.False(t, existed)
		assert.True(t, rec.Called("new"))
		assert.NotNil(t, up.session)
		assert.Equal(t, "demo", up.session.Name)
		assert.Equal(t, tmux.SessionId(1), up.session.Id)
	})

	t.Run("errors when tmux is unreachable", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Failure("error connecting to /tmp/tmux-0/default (Permission denied)"))

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.ErrorIs(t, err, tmux.ErrUnreachable)
		assert.False(t, rec.Called("new"))
	})

	t.Run("sanitises a session name tmux would rewrite and warns", func(t *testing.T) {
		up, rec := buildUp(t, strings.Replace(validProfile, `"demo"`, `"a.b\\c"`, 1), map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))
		rec.On("new", tmuxtest.Result{Output: "$1;a-b-c;/tmp"})

		var logs bytes.Buffer
		up.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.NoError(t, err)

		// Every tmux call uses the sanitised name, so a second up and down
		// find the session tmux created.
		assert.Contains(t, rec.ArgsFor("has-session"), "=a-b-c")
		assert.Subset(t, rec.ArgsFor("new"), []string{"-s", "a-b-c"})

		assert.Contains(t, logs.String(), "level=WARN")
		assert.Contains(t, logs.String(), `name=a.b\c`)
		assert.Contains(t, logs.String(), "tmux_name=a-b-c")
	})

	t.Run("does not warn about a session name tmux accepts", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))
		rec.On("new", tmuxtest.Result{Output: "$1;demo;/tmp"})

		var logs bytes.Buffer
		up.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.NoError(t, err)
		assert.NotContains(t, logs.String(), "level=WARN")
	})

	t.Run("attaches to an existing session when not detached", func(t *testing.T) {
		t.Setenv("TMUX", "")
		t.Cleanup(tmux.OverrideTerminalCheck(func() bool { return true }))
		up, rec := buildUp(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("attach", tmuxtest.Result{})

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		existed, err := up.resolveSession(profile)
		assert.NoError(t, err)
		assert.True(t, existed)
		assert.True(t, rec.Called("attach"))
		assert.True(t, rec.Called("ls"))
	})

	t.Run("shows the attach command inside another tmux server", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-501/work,1234,0")
		t.Setenv("TMUX_PANE", "%4")
		up, rec := buildUp(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})

		var logs bytes.Buffer
		up.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.NoError(t, err)
		assert.False(t, rec.Called("attach"))
		assert.False(t, rec.Called("switchc"))
		assert.Contains(t, logs.String(), "does not attach")
		assert.Contains(t, logs.String(), "tmux attach -t '=demo'")
	})

	t.Run("shows the attach command without a terminal", func(t *testing.T) {
		t.Setenv("TMUX", "")
		t.Cleanup(tmux.OverrideTerminalCheck(func() bool { return false }))
		up, rec := buildUp(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})

		var logs bytes.Buffer
		up.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.NoError(t, err)
		assert.False(t, rec.Called("attach"))
		assert.Contains(t, logs.String(), "no terminal to attach to")
		assert.Contains(t, logs.String(), "tmux attach -t '=demo'")
	})

	t.Run("finds existing session without attaching when detached", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		existed, err := up.resolveSession(profile)
		assert.NoError(t, err)
		assert.True(t, existed)
		assert.False(t, rec.Called("attach"))
		assert.False(t, rec.Called("switchc"))
	})

	t.Run("kills the previous session when --clear is set", func(t *testing.T) {
		t.Setenv("TMUX", "")
		up, rec := buildUp(t, validProfile, map[string]string{"clear": "true", "detached": "true"})
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))
		rec.On("new", tmuxtest.Result{Output: "$1;demo;/tmp"})

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.NoError(t, err)
		assert.True(t, rec.Called("kill-session"))
	})

	t.Run("refuses --clear inside the session that it would kill", func(t *testing.T) {
		insidePane(t)
		up, rec := buildUp(t, validProfile, map[string]string{"clear": "true", "detached": "true"})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Output: "$1;demo;/tmp"})

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.ErrorContains(t, err, "glaze runs inside session `demo`")
		assert.False(t, rec.Called("kill-session"))
		assert.False(t, rec.Called("new"))
	})

	t.Run("allows --clear from another session", func(t *testing.T) {
		insidePane(t)
		up, rec := buildUp(t, validProfile, map[string]string{"clear": "true", "detached": "true"})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Output: "$2;other;/srv"})
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))
		rec.On("new", tmuxtest.Result{Output: "$3;demo;/tmp"})

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.NoError(t, err)
		assert.Contains(t, rec.ArgsFor("kill-session"), "=demo")
	})

	t.Run("errors when an existing session cannot be found", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;other;/tmp"})

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not find session")
	})
}

func TestActionUpRun(t *testing.T) {
	t.Run("provisions a brand new detached session end to end", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))
		rec.On("new", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;default;tiled;1"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
		rec.On("splitw", tmuxtest.Result{Output: "%2;1;shell;1;/tmp"})

		assert.NoError(t, up.Run(context.Background()))

		// The window that tmux creates with the session becomes the first declared window.
		assert.True(t, rec.Called("new"))
		assert.Subset(t, rec.ArgsFor("renamew"), []string{"-t", "@1", "main"})
		assert.False(t, rec.Called("neww"))
		assert.True(t, rec.Called("splitw"))
		assert.False(t, rec.Called("killw"))
		// Detached: no attach/switch should be issued.
		assert.False(t, rec.Called("attach"))
		assert.False(t, rec.Called("ls"))
		assert.False(t, rec.Called("switchc"))
		assert.False(t, rec.Called("kill-session"))
	})

	// failingProvision queues the replies for a new session whose first split fails.
	failingProvision := func(rec *tmuxtest.Recorder, onSplit func()) {
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))
		rec.On("new", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;default;tiled;1"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
		rec.On("splitw", tmuxtest.Failure("no space for new pane").With(onSplit))
	}

	t.Run("removes the session it created when provisioning fails", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		failingProvision(rec, nil)

		err := up.Run(context.Background())
		assert.ErrorContains(t, err, "failed to provision session `demo`")
		assert.ErrorContains(t, err, "no space for new pane")
		assert.Equal(t, []string{"kill-session", "-t", "$1"}, rec.ArgsFor("kill-session"))
	})

	t.Run("says that the session ended when it is gone after a failure", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		failingProvision(rec, nil)
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))

		err := up.Run(context.Background())
		assert.ErrorContains(t, err, "session `demo` ended while glaze set it up")
		assert.ErrorContains(t, err, "no space for new pane")
		assert.False(t, rec.Called("kill-session"))
	})

	t.Run("keeps the session with --keep-on-failure", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true", "keep-on-failure": "true"})
		failingProvision(rec, nil)

		var logs bytes.Buffer
		up.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		assert.Error(t, up.Run(context.Background()))
		assert.False(t, rec.Called("kill-session"))
		assert.Contains(t, logs.String(), "kept the partly built session")
	})

	t.Run("removes the session it created when the run is cancelled", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		ctx, cancel := context.WithCancel(context.Background())
		failingProvision(rec, cancel)

		assert.Error(t, up.Run(ctx))
		assert.Equal(t, []string{"kill-session", "-t", "$1"}, rec.ArgsFor("kill-session"))
	})

	t.Run("uses a session that another run created first", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))
		rec.On("new", tmuxtest.Failure("duplicate session: demo"))
		rec.On("ls", tmuxtest.Result{Output: "$4;demo;/tmp"})

		assert.NoError(t, up.Run(context.Background()))
		assert.Equal(t, tmux.SessionId(4), up.session.Id)
		assert.False(t, rec.Called("splitw"))
		assert.False(t, rec.Called("kill-session"))
	})

	t.Run("returns early without provisioning when already attached", func(t *testing.T) {
		t.Setenv("TMUX", "")
		up, rec := buildUp(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("attach", tmuxtest.Result{})

		assert.NoError(t, up.Run(context.Background()))
		// An existing session must not be re-provisioned (no new windows/panes).
		assert.False(t, rec.Called("neww"))
		assert.False(t, rec.Called("splitw"))
		assert.False(t, rec.Called("killw"))
	})

	t.Run("leaves a detached existing session untouched", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})

		assert.NoError(t, up.Run(context.Background()))
		assert.False(t, rec.Called("neww"))
		assert.False(t, rec.Called("attach"))
		assert.False(t, rec.Called("switchc"))
	})

	t.Run("propagates profile load errors", func(t *testing.T) {
		bad := `session {
  name = "demo"
  window {
    name   = "main"
    layout = "noop"
    pane { name = "shell" }
  }
}
`
		up, _ := buildUp(t, bad, map[string]string{"detached": "true"})
		assert.ErrorIs(t, up.Run(context.Background()), diagnostics.ErrHasDiagnostics)
	})
}
