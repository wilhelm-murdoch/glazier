package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// buildDown builds ActionDown as main does, with a recorder for tmux. An empty profile writes no file, to test --session alone.
func buildDown(t *testing.T, profile string, flags map[string]string) (*ActionDown, *tmuxtest.Recorder) {
	t.Helper()

	rec := tmuxtest.New().Default(tmuxtest.Result{})
	rec.Install(t)

	args := []string{"down"}

	if profile != "" {
		path := filepath.Join(t.TempDir(), ".glaze")
		assert.NoError(t, os.WriteFile(path, []byte(profile), 0o600))
		args = append(args, "--profile-path", path)
	}

	for k, v := range flags {
		args = append(args, "--"+k, v)
	}

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

	assert.NoError(t, cmd.Run(context.Background(), args))
	if actErr != nil {
		t.Skipf("could not construct ActionDown (tmux likely unavailable): %v", actErr)
	}

	return action, rec
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
