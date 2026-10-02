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
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
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
			&cli.BoolFlag{Name: "force"},
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

	t.Run("refuses to replace an existing profile without --force, before the capture", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".glaze")
		assert.NoError(t, os.WriteFile(path, []byte("# hand-written\n"), 0o600))

		save, rec := buildSave(t, map[string]string{"profile-path": path})
		insidePane(t)
		rec.On("list-sessions", tmuxtest.Result{})

		err := save.Run(context.Background())
		assert.ErrorIs(t, err, files.ErrFileExists)
		assert.ErrorContains(t, err, "use --force")
		assert.False(t, rec.Called("lsw"))

		contents, err := os.ReadFile(path) //nolint:gosec // G304
		assert.NoError(t, err)
		assert.Equal(t, "# hand-written\n", string(contents))
	})

	t.Run("replaces an existing profile with --force, through a symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "real.glaze")
		link := filepath.Join(dir, ".glaze")
		assert.NoError(t, os.WriteFile(target, []byte("# hand-written\n"), 0o600))
		assert.NoError(t, os.Symlink(target, link))

		save, rec := buildSave(t, map[string]string{"profile-path": link, "force": "true"})
		insidePane(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;shell;1;/tmp"})

		assert.NoError(t, save.Run(context.Background()))

		info, err := os.Lstat(link)
		assert.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink, "the symlink must stay")

		contents, err := os.ReadFile(target) //nolint:gosec // G304
		assert.NoError(t, err)
		assert.Contains(t, string(contents), `"demo"`)
	})

	t.Run("leaves out names that tmux chose and directories that no longer exist", func(t *testing.T) {
		dir := t.TempDir()
		gone := filepath.Join(dir, "gone")
		save, rec := buildSave(t, map[string]string{"session": "raw", "stdout": "true"})

		var logs bytes.Buffer
		save.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;raw;" + dir})
		rec.On("display-message", tmuxtest.Result{Output: "buildhost"}) // #{host}
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;zsh;tiled;1\n@2;2;logs;tiled;0"})
		rec.On("display-message", tmuxtest.Result{Output: "1"}) // @1 has automatic-rename on
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;buildhost;1;" + gone})
		rec.On("display-message", tmuxtest.Result{Output: "0"}) // @2 was named
		rec.On("lsp", tmuxtest.Result{Output: "%2;1;tail;1;" + dir})

		session, err := save.resolveSession()
		require.NoError(t, err)

		captured, err := save.captureSession(session)
		require.NoError(t, err)

		assert.Empty(t, captured.Windows[0].Name, "tmux named this window after its program")
		assert.Empty(t, captured.Windows[0].Panes[0].Name, "the host name is the default title of a pane")
		assert.Empty(t, captured.Windows[0].Panes[0].StartingDirectory, "the directory no longer exists")
		assert.Equal(t, "logs", captured.Windows[1].Name)
		assert.Equal(t, "tail", captured.Windows[1].Panes[0].Name)
		assert.Equal(t, dir, captured.Windows[1].Panes[0].StartingDirectory)
		assert.Contains(t, logs.String(), "the directory no longer exists")
		assert.Contains(t, logs.String(), gone)

		profile := string(generateProfile(captured))
		assert.NotContains(t, profile, `"zsh"`)
		assert.NotContains(t, profile, `"buildhost"`)
		assert.NotContains(t, profile, gone)
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
		// --session bypasses the current-session lookup, which reads the socket path and the session of the pane.
		for _, call := range rec.Calls {
			if call[0] == "display-message" {
				assert.NotContains(t, strings.Join(call, " "), "socket_path")
				assert.NotContains(t, strings.Join(call, " "), "session_id")
			}
		}
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
