package actions

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// buildDown builds ActionDown as main does, with a recorder for tmux. An empty profile writes no file, to test --session alone.
func buildDown(t *testing.T, profile string, flags map[string]string) (*ActionDown, *tmuxtest.Recorder) {
	t.Helper()

	var args []string
	if profile != "" {
		path := filepath.Join(t.TempDir(), ".glaze")
		assert.NoError(t, os.WriteFile(path, []byte(profile), 0o600))
		args = append(args, "--profile-path", path)
	}

	action, rec, err := newDownWith(t, append(args, flagArgs(flags)...)...)
	require.NoError(t, err)

	return action, rec
}

// buildDownByName writes the profile into a new profile registry and builds ActionDown for its name, as `glaze down <name>` does.
func buildDownByName(t *testing.T, name, profile string, flags map[string]string) (*ActionDown, *tmuxtest.Recorder) {
	t.Helper()

	writeRegistryProfile(t, newRegistry(t), name, profile)

	action, rec, err := newDownWith(t, append([]string{name}, flagArgs(flags)...)...)
	require.NoError(t, err)

	return action, rec
}

// newDownWith runs NewDown with the arguments of down and returns its result. It skips the test when tmux is not installed.
func newDownWith(t *testing.T, args ...string) (*ActionDown, *tmuxtest.Recorder, error) {
	t.Helper()

	rec := tmuxtest.New().Default(tmuxtest.Result{})
	rec.Install(t)

	var (
		action *ActionDown
		actErr error
	)

	cmd := &cli.Command{
		Name: "down",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "session"},
			&cli.StringFlag{Name: "socket-path"},
			&cli.StringFlag{Name: "socket-name"},
			&cli.StringFlag{Name: "profile-path"},
			&cli.StringSliceFlag{Name: "var"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			action, actErr = NewDown(c, "critical")
			return nil
		},
	}

	assert.NoError(t, cmd.Run(context.Background(), append([]string{"down"}, args...)))
	if errors.Is(actErr, tmux.ErrUnreachable) {
		t.Skipf("could not construct ActionDown (tmux likely unavailable): %v", actErr)
	}

	return action, rec, actErr
}

func TestActionDownRun(t *testing.T) {
	t.Run("kills the session named by the profile", func(t *testing.T) {
		down, rec := buildDown(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))

		assert.True(t, rec.Called("kill-session"))
		assert.Contains(t, rec.ArgsFor("kill-session"), "=demo")
	})

	t.Run("names the session the same way as up", func(t *testing.T) {
		for nameLine, want := range map[string]string{
			"name = 42":   "=42",
			"name = true": "=true",
			"":            "=default",
		} {
			profile := "session {\n  " + nameLine + "\n  window {\n    pane {}\n  }\n}\n"
			down, rec := buildDown(t, profile, nil)
			rec.On("has-session", tmuxtest.Result{})
			rec.On("kill-session", tmuxtest.Result{})

			assert.NoError(t, down.Run(context.Background()), nameLine)
			assert.Contains(t, rec.ArgsFor("kill-session"), want, nameLine)
		}
	})

	t.Run("refuses random() in the session name and kills nothing", func(t *testing.T) {
		profile := "session {\n  name = random([\"a\", \"b\"])\n  window {\n    pane {}\n  }\n}\n"
		down, rec := buildDown(t, profile, nil)

		assert.ErrorIs(t, down.Run(context.Background()), diagnostics.ErrHasDiagnostics)
		assert.False(t, rec.Called("has-session"))
		assert.False(t, rec.Called("kill-session"))
	})

	t.Run("kills the session named by --session without a profile", func(t *testing.T) {
		down, rec := buildDown(t, "", map[string]string{"session": "other"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))

		assert.True(t, rec.Called("kill-session"))
		assert.Contains(t, rec.ArgsFor("kill-session"), "=other")
	})

	t.Run("--session wins over the profile", func(t *testing.T) {
		down, rec := buildDown(t, validProfile, map[string]string{"session": "other"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))

		assert.Contains(t, rec.ArgsFor("kill-session"), "=other")
	})

	t.Run("errors when tmux is unreachable", func(t *testing.T) {
		down, rec := buildDown(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Failure("error connecting to /tmp/tmux-0/default (Permission denied)"))

		err := down.Run(context.Background())
		assert.ErrorIs(t, err, tmux.ErrUnreachable)
		assert.ErrorContains(t, err, "could not check for session `demo`")
		assert.False(t, rec.Called("kill-session"))
	})

	t.Run("is a no-op when the session is not running", func(t *testing.T) {
		down, rec := buildDown(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))

		assert.NoError(t, down.Run(context.Background()))

		assert.False(t, rec.Called("kill-session"))
	})

	t.Run("resolves an interpolated session name from --var", func(t *testing.T) {
		profile := `variable "district" {
  type = string
}

session {
  name = "gig-${var.district}"

  window {
    pane {}
  }
}
`
		down, rec := buildDown(t, profile, map[string]string{"var": "district=watson"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))

		assert.Contains(t, rec.ArgsFor("kill-session"), "=gig-watson")
	})

	t.Run("ignores variables used only deeper in the profile", func(t *testing.T) {
		// greeting has no default and no value, but `down` evaluates only `name`, so a variable that only a pane uses is not needed.
		profile := `variable "greeting" {
  type = string
}

session {
  name = "demo"

  window {
    pane {
      commands = ["echo ${var.greeting}"]
    }
  }
}
`
		down, rec := buildDown(t, profile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))

		assert.Contains(t, rec.ArgsFor("kill-session"), "=demo")
	})

	t.Run("propagates kill failures", func(t *testing.T) {
		down, rec := buildDown(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{Err: assert.AnError})

		err := down.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not bring down session")
	})
}

func TestActionDownByName(t *testing.T) {
	t.Run("kills the session that the registry profile names", func(t *testing.T) {
		down, rec := buildDownByName(t, "work", validProfile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))

		// The profile name is a file key, so the session is the one that `up work` created.
		assert.Contains(t, rec.ArgsFor("has-session"), "=demo")
		assert.Contains(t, rec.ArgsFor("kill-session"), "=demo")
		for _, call := range rec.Calls {
			assert.NotContains(t, call, "=work")
		}
	})

	t.Run("resolves an interpolated name of a registry profile from --var", func(t *testing.T) {
		profile := `variable "district" {
  type = string
}

session {
  name = "gig-${var.district}"

  window {
    pane {}
  }
}
`
		down, rec := buildDownByName(t, "gig", profile, map[string]string{"var": "district=watson"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))

		assert.Contains(t, rec.ArgsFor("has-session"), "=gig-watson")
		assert.Contains(t, rec.ArgsFor("kill-session"), "=gig-watson")
	})

	t.Run("says how to kill the session of a profile that is gone", func(t *testing.T) {
		registry := newRegistry(t)
		writeRegistryProfile(t, registry, "other", validProfile)

		down, rec, err := newDownWith(t, "nope")

		assert.Nil(t, down)
		assert.ErrorIs(t, err, files.ErrProfileNotFound)
		assert.ErrorContains(t, err, "no profile named `nope` in the profile registry")
		assert.ErrorContains(t, err, "\n - to kill a session by its tmux name, use --session")
		assert.Empty(t, rec.Calls)
	})

	t.Run("says how to kill the session when the profile registry cannot be used", func(t *testing.T) {
		newRegistry(t)
		t.Setenv("GLAZE_REGISTRY", "rel")

		down, rec, err := newDownWith(t, "work")

		// The registry error carries both sentinels, so `down` exits 3 and still names the way to the session.
		assert.Nil(t, down)
		assert.ErrorIs(t, err, files.ErrProfileNotFound)
		assert.ErrorIs(t, err, files.ErrRegistryUnusable)
		assert.ErrorContains(t, err, "GLAZE_REGISTRY must be an absolute path, but it is `rel`")
		assert.ErrorContains(t, err, "\n - to kill a session by its tmux name, use --session")
		assert.Empty(t, rec.Calls)
	})

	t.Run("keeps the plain message for a path that does not exist", func(t *testing.T) {
		newRegistry(t)

		down, _, err := newDownWith(t, "--profile-path", filepath.Join(t.TempDir(), "missing.glaze"))

		assert.Nil(t, down)
		assert.ErrorIs(t, err, files.ErrProfileNotFound)
		assert.NotContains(t, err.Error(), "use --session")
	})

	t.Run("builds no base for --session without a profile registry", func(t *testing.T) {
		newRegistry(t)
		t.Chdir(t.TempDir())

		down, rec := buildDown(t, "", map[string]string{"session": "other"})
		assert.Nil(t, down.base)

		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))
		assert.Contains(t, rec.ArgsFor("kill-session"), "=other")
	})
}

func TestActionDownByNameChecks(t *testing.T) {
	t.Run("rejects a registry profile without a session name and kills nothing", func(t *testing.T) {
		down, rec := buildDownByName(t, "work", noNameProfile, nil)
		diags := captureDiagnostics(down.base)

		assert.ErrorIs(t, down.Run(context.Background()), diagnostics.ErrHasDiagnostics)

		// The session would be default, which another profile may own.
		assert.Contains(t, diags.String(), "Error: Missing session name")
		assert.Contains(t, diags.String(), "`glaze down work` could kill an unrelated session")
		assert.False(t, rec.Called("has-session"))
		assert.False(t, rec.Called("kill-session"))
	})

	t.Run("kills the default session of a path profile without a session name", func(t *testing.T) {
		down, rec := buildDown(t, noNameProfile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))
		assert.Contains(t, rec.ArgsFor("kill-session"), "=default")
	})

	t.Run("says that a name from path changes with the directory when the session is not running", func(t *testing.T) {
		project := filepath.Join(t.TempDir(), "proj")
		assert.NoError(t, os.Mkdir(project, 0o700))
		t.Chdir(project)

		down, rec := buildDownByName(t, "reg", templateProfile, nil)
		rec.On("has-session", tmuxtest.Failure("can't find session: reg-proj"))

		var logs bytes.Buffer
		down.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		assert.NoError(t, down.Run(context.Background()))

		assert.Contains(t, rec.ArgsFor("has-session"), "=reg-proj")
		assert.False(t, rec.Called("kill-session"))
		assert.Contains(t, logs.String(), "the name depends on the current directory, so from another directory use --session")
		assert.Contains(t, logs.String(), "session=reg-proj")
	})

	t.Run("kills the session of a name from path in the directory that named it", func(t *testing.T) {
		project := filepath.Join(t.TempDir(), "proj")
		assert.NoError(t, os.Mkdir(project, 0o700))
		t.Chdir(project)

		down, rec := buildDownByName(t, "reg", templateProfile, nil)
		rec.On("has-session", tmuxtest.Result{})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run(context.Background()))
		assert.Contains(t, rec.ArgsFor("kill-session"), "=reg-proj")
	})

	t.Run("keeps the plain message for a fixed name that is not running", func(t *testing.T) {
		down, rec := buildDownByName(t, "work", validProfile, nil)
		rec.On("has-session", tmuxtest.Failure("can't find session: demo"))

		var logs bytes.Buffer
		down.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		assert.NoError(t, down.Run(context.Background()))

		assert.Contains(t, logs.String(), "nothing to do; session is not running")
		assert.NotContains(t, logs.String(), "depends on the current directory")
	})
}
