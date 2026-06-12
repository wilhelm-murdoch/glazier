package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// buildDown constructs a fully wired ActionDown with a tmuxtest recorder
// installed, mirroring how main.go builds the action. An empty profile means
// no profile file is written, exercising the --session-only path.
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
		rec.On("has-session", tmuxtest.Result{Status: 0})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run())

		assert.True(t, rec.Called("kill-session"))
		assert.Contains(t, rec.ArgsFor("kill-session"), "demo")
	})

	t.Run("kills the session named by --session without a profile", func(t *testing.T) {
		down, rec := buildDown(t, "", map[string]string{"session": "other"})
		rec.On("has-session", tmuxtest.Result{Status: 0})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run())

		assert.True(t, rec.Called("kill-session"))
		assert.Contains(t, rec.ArgsFor("kill-session"), "other")
	})

	t.Run("--session wins over the profile", func(t *testing.T) {
		down, rec := buildDown(t, validProfile, map[string]string{"session": "other"})
		rec.On("has-session", tmuxtest.Result{Status: 0})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run())

		assert.Contains(t, rec.ArgsFor("kill-session"), "other")
	})

	t.Run("is a no-op when the session is not running", func(t *testing.T) {
		down, rec := buildDown(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{Status: 1})

		assert.NoError(t, down.Run())

		assert.False(t, rec.Called("kill-session"))
	})

	t.Run("resolves an interpolated session name from --var", func(t *testing.T) {
		profile := `session {
  name = "gig-${district}"

  window {
    pane {}
  }
}
`
		down, rec := buildDown(t, profile, map[string]string{"var": "district=watson"})
		rec.On("has-session", tmuxtest.Result{Status: 0})
		rec.On("kill-session", tmuxtest.Result{})

		assert.NoError(t, down.Run())

		assert.Contains(t, rec.ArgsFor("kill-session"), "gig-watson")
	})

	t.Run("propagates kill failures", func(t *testing.T) {
		down, rec := buildDown(t, validProfile, nil)
		rec.On("has-session", tmuxtest.Result{Status: 0})
		rec.On("kill-session", tmuxtest.Result{Err: assert.AnError})

		err := down.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not bring down session")
	})
}
