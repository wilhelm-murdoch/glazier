package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// buildSave constructs an ActionSave with a tmuxtest recorder installed and the
// given save-command flags applied.
func buildSave(t *testing.T, flags map[string]string) (*ActionSave, *tmuxtest.Recorder) {
	t.Helper()

	rec := tmuxtest.New().Default(tmuxtest.Result{})
	rec.Install(t)

	var (
		action *ActionSave
		actErr error
	)

	cmd := &cli.Command{
		Name: "save",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "socket-path"},
			&cli.StringFlag{Name: "socket-name"},
			&cli.StringFlag{Name: "profile-path"},
			&cli.StringFlag{Name: "session"},
			&cli.BoolFlag{Name: "stdout"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			action, actErr = NewSave(c, "critical")
			return nil
		},
	}

	args := []string{"save"}
	for k, v := range flags {
		args = append(args, "--"+k, v)
	}

	assert.NoError(t, cmd.Run(context.Background(), args))
	if actErr != nil {
		t.Skipf("could not construct ActionSave (tmux likely unavailable): %v", actErr)
	}

	return action, rec
}

// insidePane sets the environment of pane %4 of the tmux server on /tmp/tmux-501/default.
func insidePane(t *testing.T) {
	t.Helper()
	t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
	t.Setenv("TMUX_PANE", "%4")
}

func TestActionSaveRun(t *testing.T) {
	t.Run("errors when no tmux server is running", func(t *testing.T) {
		save, rec := buildSave(t, nil)
		rec.On("list-sessions", tmuxtest.Failure("no server running on /tmp/tmux-1000/default"))

		err := save.Run(context.Background())
		assert.ErrorContains(t, err, "no tmux server is running")
		assert.NotErrorIs(t, err, tmux.ErrUnreachable)
	})

	t.Run("errors when tmux is unreachable", func(t *testing.T) {
		save, rec := buildSave(t, nil)
		rec.On("list-sessions", tmuxtest.Failure("error connecting to /tmp/tmux-0/default (Permission denied)"))

		assert.ErrorIs(t, save.Run(context.Background()), tmux.ErrUnreachable)
	})

	t.Run("captures the current session and writes a profile", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "out.glaze")
		save, rec := buildSave(t, map[string]string{"profile-path": path})
		insidePane(t)

		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;bb62,80x24,0,0;1"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;shell;1;/tmp"})

		assert.NoError(t, save.Run(context.Background()))

		// The current session comes from display-message, with no second
		// lookup by name.
		assert.False(t, rec.Called("ls"))

		// The path points at this test's own temp directory.
		contents, err := os.ReadFile(path) //nolint:gosec // G304
		assert.NoError(t, err)
		assert.Contains(t, string(contents), `"demo"`)
		assert.Contains(t, string(contents), `"main"`)
		assert.Contains(t, string(contents), `"shell"`)
		// The active window and pane (trailing `1`) are captured as focused.
		assert.Contains(t, string(contents), "focus")
		assert.Contains(t, string(contents), "= true")
		// tmux reports no preset, so the raw layout string is captured as it is and must still validate on replay.
		assert.Contains(t, string(contents), `layout = "bb62,80x24,0,0"`)
	})

	t.Run("does not mark inactive windows or panes as focused", func(t *testing.T) {
		save, rec := buildSave(t, map[string]string{"stdout": "true"})
		insidePane(t)

		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;0"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;shell;0;/tmp"})

		// Capturing through the public model so we can assert on the result.
		session, err := save.resolveSession()
		assert.NoError(t, err)

		captured, err := save.captureSession(session)
		assert.NoError(t, err)
		assert.False(t, captured.Windows[0].Focus)
		assert.False(t, captured.Windows[0].Panes[0].Focus)
	})

	t.Run("needs --session outside tmux", func(t *testing.T) {
		t.Setenv("TMUX", "")
		save, rec := buildSave(t, map[string]string{"stdout": "true"})
		rec.On("list-sessions", tmuxtest.Result{})

		assert.ErrorContains(t, save.Run(context.Background()), "--session")
		assert.False(t, rec.Called("display-message"))
		assert.False(t, rec.Called("lsw"))
	})

	t.Run("needs --session inside another tmux server", func(t *testing.T) {
		save, rec := buildSave(t, map[string]string{"stdout": "true"})
		insidePane(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/work"})

		assert.ErrorContains(t, save.Run(context.Background()), "--session")
		assert.False(t, rec.Called("lsw"))
	})

	t.Run("honours the --session flag", func(t *testing.T) {
		save, rec := buildSave(t, map[string]string{"session": "other", "stdout": "true"})

		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$2;other;/srv"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;p;1;/srv"})

		assert.NoError(t, save.Run(context.Background()))
		// --session bypasses the current-session lookup.
		assert.False(t, rec.Called("display-message"))
	})

	t.Run("errors when the target session cannot be found", func(t *testing.T) {
		save, rec := buildSave(t, map[string]string{"session": "ghost"})

		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})

		err := save.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
