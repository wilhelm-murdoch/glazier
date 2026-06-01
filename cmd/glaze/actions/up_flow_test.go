package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
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

// buildUp constructs a fully wired ActionUp from the given profile contents and
// up-command flags, with a tmuxtest recorder installed. It mirrors how main.go
// builds the action so the real parser, diagnostics manager and tmux client are
// exercised end to end (minus a live tmux server).
func buildUp(t *testing.T, profile string, flags map[string]string) (*ActionUp, *tmuxtest.Recorder) {
	t.Helper()

	rec := tmuxtest.New().Default(tmuxtest.Result{Output: "base-index 1"})
	rec.Install(t)

	dir := t.TempDir()
	path := filepath.Join(dir, ".glaze")
	assert.NoError(t, os.WriteFile(path, []byte(profile), 0o644))

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

func TestActionUpLoadProfile(t *testing.T) {
	t.Run("decodes a valid profile", func(t *testing.T) {
		up, _ := buildUp(t, validProfile, nil)

		profile, err := up.loadProfile()
		assert.NoError(t, err)
		assert.NotNil(t, profile)
		assert.Equal(t, "demo", profile.Name)
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
		rec.On("has-session", tmuxtest.Result{Status: 1})
		rec.On("new", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		existed, err := up.resolveSession(profile)
		assert.NoError(t, err)
		assert.False(t, existed)
		assert.True(t, rec.Called("new"))
		assert.NotNil(t, up.session)
		assert.Equal(t, "demo", up.session.Name)
	})

	t.Run("attaches to an existing session when not detached", func(t *testing.T) {
		t.Setenv("TMUX", "")
		up, rec := buildUp(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{Status: 0})
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

	t.Run("finds existing session without attaching when detached", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Result{Status: 0})
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
		up, rec := buildUp(t, validProfile, map[string]string{"clear": "true", "detached": "true"})
		rec.On("has-session", tmuxtest.Result{Status: 1})
		rec.On("new", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})

		profile, err := up.loadProfile()
		assert.NoError(t, err)

		_, err = up.resolveSession(profile)
		assert.NoError(t, err)
		assert.True(t, rec.Called("kill-session"))
	})

	t.Run("errors when an existing session cannot be found", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Result{Status: 0})
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
		rec.On("has-session", tmuxtest.Result{Status: 1})
		rec.On("new", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("neww", tmuxtest.Result{Output: "@1;1;main;tiled;1"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
		rec.On("splitw", tmuxtest.Result{Output: "%2;1;shell;1"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;default;tiled;1"})

		assert.NoError(t, up.Run())

		assert.True(t, rec.Called("new"))
		assert.True(t, rec.Called("neww"))
		assert.True(t, rec.Called("splitw"))
		assert.True(t, rec.Called("killw"))
		// Detached: no attach/switch should be issued.
		assert.False(t, rec.Called("attach"))
		assert.False(t, rec.Called("switchc"))
	})

	t.Run("returns early without provisioning when already attached", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{Status: 0})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("attach", tmuxtest.Result{})

		assert.NoError(t, up.Run())
		// An existing session must not be re-provisioned (no new windows/panes).
		assert.False(t, rec.Called("neww"))
		assert.False(t, rec.Called("splitw"))
		assert.False(t, rec.Called("killw"))
	})

	t.Run("leaves a detached existing session untouched", func(t *testing.T) {
		up, rec := buildUp(t, validProfile, map[string]string{"detached": "true"})
		rec.On("has-session", tmuxtest.Result{Status: 0})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})

		assert.NoError(t, up.Run())
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
		assert.ErrorIs(t, up.Run(), diagnostics.ErrHasDiagnostics)
	})
}
