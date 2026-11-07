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
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/spec"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// buildSave constructs an ActionSave with a tmuxtest recorder installed and the
// given save-command flags applied.
func buildSave(t *testing.T, flags map[string]string) (*ActionSave, *tmuxtest.Recorder) {
	t.Helper()

	return buildSaveAt(t, "critical", "", flags)
}

// buildSaveAt is buildSave with the log level of the action and a positional profile name, which goes after the flags as it does from main.
func buildSaveAt(t *testing.T, logLevel, name string, flags map[string]string) (*ActionSave, *tmuxtest.Recorder) {
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
			action, actErr = NewSave(c, logLevel)
			return nil
		},
	}

	args := append([]string{"save"}, flagArgs(flags)...)
	if name != "" {
		args = append(args, name)
	}

	assert.NoError(t, cmd.Run(context.Background(), args))
	if actErr != nil {
		t.Skipf("could not construct ActionSave (tmux likely unavailable): %v", actErr)
	}

	return action, rec
}

// recordSession queues the replies for a capture of the session `sv` through --session, with one window and one pane in dir.
func recordSession(rec *tmuxtest.Recorder, dir string) {
	rec.On("list-sessions", tmuxtest.Result{})
	rec.On("ls", tmuxtest.Result{Output: "$1;sv;" + dir})
	rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;shell;1;" + dir})
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
		rec.On("lsp", tmuxtest.Result{Output: "%2;1;tail;0;" + dir})

		session, err := save.resolveSession()
		require.NoError(t, err)

		captured, err := captureSession(save.tmux, session, save.Logger)
		require.NoError(t, err)
		pruneMissingDirectories(&captured, save.Logger)

		assert.Empty(t, captured.Windows[0].Name, "tmux named this window after its program")
		assert.Empty(t, captured.Windows[0].Panes[0].Name, "the host name is the default title of a pane")
		assert.Empty(t, captured.Windows[0].Panes[0].StartingDirectory, "the directory no longer exists")
		assert.Equal(t, "logs", captured.Windows[1].Name)
		assert.Equal(t, "tail", captured.Windows[1].Panes[0].Name)
		assert.Equal(t, dir, captured.Windows[1].Panes[0].StartingDirectory)
		assert.False(t, captured.Windows[1].Focus)
		assert.False(t, captured.Windows[1].Panes[0].Focus)
		assert.Contains(t, logs.String(), "the directory no longer exists")
		assert.Contains(t, logs.String(), gone)

		profile := string(generateProfile(captured))
		assert.NotContains(t, profile, `"zsh"`)
		assert.NotContains(t, profile, `"buildhost"`)
		assert.NotContains(t, profile, gone)
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
}

func TestActionSaveRunWithAProfileName(t *testing.T) {
	t.Run("creates the registry with mode 0700 and writes a profile that round-trips", func(t *testing.T) {
		registry := newRegistry(t)
		dir := t.TempDir()
		save, rec := buildSaveAt(t, "critical", "work", map[string]string{"session": "sv"})
		recordSession(rec, dir)

		require.NoError(t, save.Run(context.Background()))
		// A name with --session skips the lookup of the current session.
		for _, call := range rec.Calls {
			if call[0] == "display-message" {
				assert.NotContains(t, strings.Join(call, " "), "session_id")
			}
		}

		// MkdirAll with 0700 cannot lose a bit under any umask; the file mode can, so only its guaranteed bits are checked.
		info, err := os.Stat(registry)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

		path := filepath.Join(registry, "work.glaze")
		fileInfo, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm()&0o600)
		assert.Zero(t, fileInfo.Mode().Perm()&^0o644)

		p, diags := parser.New(path)
		require.False(t, diags.HasErrors(), diags.Error())

		session, decodeDiags := p.Decode(spec.Session(registry), parser.BuildEvalContext(map[string]cty.Value{}))
		require.False(t, decodeDiags.HasErrors(), decodeDiags.Error())
		require.NotNil(t, session)

		assert.Equal(t, "sv", session.Name)
		assert.Equal(t, dir, session.StartingDirectory)
		require.Len(t, session.Windows, 1)
		assert.Equal(t, "main", session.Windows[0].Name)
		assert.True(t, session.Windows[0].Focus)
		require.Len(t, session.Windows[0].Panes, 1)
		assert.Equal(t, "shell", session.Windows[0].Panes[0].Name)
		assert.True(t, session.Windows[0].Panes[0].Focus)
	})

	t.Run("logs the absolute path of the registry profile", func(t *testing.T) {
		registry := newRegistry(t)
		dir := t.TempDir()

		var err error
		stderr := capture(t, &os.Stderr, func() {
			save, rec := buildSaveAt(t, "info", "work", map[string]string{"session": "sv"})
			recordSession(rec, dir)
			err = save.Run(context.Background())
		})

		require.NoError(t, err)
		path := filepath.Join(registry, "work.glaze")
		assert.Contains(t, stderr, "saving session")
		assert.Contains(t, stderr, "saved session")
		assert.Equal(t, 2, strings.Count(stderr, "path="+path), stderr)
	})

	t.Run("refuses a second save of the name before the capture", func(t *testing.T) {
		registry := newRegistry(t)
		path := writeRegistryProfile(t, registry, "work", "# hand-written\n")
		save, rec := buildSaveAt(t, "critical", "work", map[string]string{"session": "sv"})
		rec.On("list-sessions", tmuxtest.Result{})

		err := save.Run(context.Background())
		assert.ErrorIs(t, err, files.ErrFileExists)
		assert.ErrorContains(t, err, "could not save to `"+path+"`")
		assert.ErrorContains(t, err, "use --force to replace it")
		assert.False(t, rec.Called("ls"))
		assert.False(t, rec.Called("lsw"))

		contents, err := os.ReadFile(path) //nolint:gosec // G304
		require.NoError(t, err)
		assert.Equal(t, "# hand-written\n", string(contents))
	})

	t.Run("replaces the registry profile with --force", func(t *testing.T) {
		registry := newRegistry(t)
		path := writeRegistryProfile(t, registry, "work", "# hand-written\n")
		dir := t.TempDir()
		save, rec := buildSaveAt(t, "critical", "work", map[string]string{"session": "sv", "force": "true"})
		recordSession(rec, dir)

		require.NoError(t, save.Run(context.Background()))

		contents, err := os.ReadFile(path) //nolint:gosec // G304
		require.NoError(t, err)
		assert.Contains(t, string(contents), `"sv"`)
		assert.NotContains(t, string(contents), "hand-written")
	})

	t.Run("prints the profile of the name with --stdout and creates nothing", func(t *testing.T) {
		registry := newRegistry(t)
		dir := t.TempDir()
		save, rec := buildSaveAt(t, "critical", "work", map[string]string{"session": "sv", "stdout": "true"})
		recordSession(rec, dir)

		var err error
		stdout := captureStdout(t, func() { err = save.Run(context.Background()) })

		require.NoError(t, err)
		assert.Contains(t, stdout, `"sv"`)
		assert.Contains(t, stdout, `"shell"`)
		assert.NoDirExists(t, registry)
		assert.NoDirExists(t, filepath.Dir(registry))
		assert.NoFileExists(t, filepath.Join(registry, "work.glaze"))
	})

	t.Run("creates no directory when the capture fails", func(t *testing.T) {
		registry := newRegistry(t)
		save, rec := buildSaveAt(t, "critical", "work", map[string]string{"session": "nope"})
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})

		err := save.Run(context.Background())
		assert.ErrorContains(t, err, "not found")
		assert.NoDirExists(t, registry)
		assert.NoDirExists(t, filepath.Dir(registry))
	})

	t.Run("fails before the capture on a relative GLAZE_REGISTRY", func(t *testing.T) {
		newRegistry(t)
		t.Setenv("GLAZE_REGISTRY", "rel")
		save, rec := buildSaveAt(t, "critical", "work", map[string]string{"session": "sv"})
		rec.On("list-sessions", tmuxtest.Result{})

		err := save.Run(context.Background())
		assert.ErrorIs(t, err, files.ErrRegistryUnusable)
		assert.ErrorContains(t, err, "GLAZE_REGISTRY must be an absolute path, but it is `rel`")
		assert.False(t, rec.Called("ls"))
		assert.False(t, rec.Called("lsw"))
	})

	t.Run("fails after the capture and before the write on a registry under a file", func(t *testing.T) {
		newRegistry(t)
		parent := filepath.Join(t.TempDir(), "parentfile")
		require.NoError(t, os.WriteFile(parent, []byte("x"), 0o600))
		t.Setenv("XDG_CONFIG_HOME", parent)

		dir := t.TempDir()
		save, rec := buildSaveAt(t, "critical", "work", map[string]string{"session": "sv"})
		recordSession(rec, dir)

		err := save.Run(context.Background())
		assert.ErrorIs(t, err, files.ErrRegistryUnusable)
		assert.ErrorContains(t, err, "cannot be the profile registry, because a part of its path is a file; set XDG_CONFIG_HOME to a directory, or set GLAZE_REGISTRY")
		assert.True(t, rec.Called("lsw"), "the capture runs before the registry check")

		info, err := os.Stat(parent)
		require.NoError(t, err)
		assert.True(t, info.Mode().IsRegular(), "the file stays a file")
		assert.NoDirExists(t, filepath.Join(parent, "glaze"))
	})

	t.Run("does not create the parent of a --profile-path target", func(t *testing.T) {
		newRegistry(t)
		path := filepath.Join(t.TempDir(), "missing", "out.glaze")
		save, rec := buildSave(t, map[string]string{"session": "sv", "profile-path": path})
		recordSession(rec, t.TempDir())

		err := save.Run(context.Background())
		assert.ErrorContains(t, err, "could not write profile `"+path+"`")
		assert.NoDirExists(t, filepath.Dir(path))
	})

	t.Run("warns about a relative XDG_CONFIG_HOME with a name", func(t *testing.T) {
		registry := newRegistry(t)
		t.Setenv("XDG_CONFIG_HOME", "rel")
		dir := t.TempDir()

		var err error
		stderr := capture(t, &os.Stderr, func() {
			save, rec := buildSaveAt(t, "warn", "work", map[string]string{"session": "sv"})
			recordSession(rec, dir)
			err = save.Run(context.Background())
		})

		require.NoError(t, err)
		assert.Equal(t, 1, strings.Count(stderr, "WRN XDG_CONFIG_HOME is not absolute, so the profile registry is ~/.config/glaze"), stderr)
		assert.Contains(t, stderr, "XDG_CONFIG_HOME=rel")
		assert.FileExists(t, filepath.Join(registry, "work.glaze"))
	})

	t.Run("does not warn about a relative XDG_CONFIG_HOME for a path", func(t *testing.T) {
		newRegistry(t)
		t.Setenv("XDG_CONFIG_HOME", "rel")
		dir := t.TempDir()
		path := filepath.Join(dir, "out.glaze")

		var err error
		stderr := capture(t, &os.Stderr, func() {
			save, rec := buildSaveAt(t, "warn", "", map[string]string{"session": "sv", "profile-path": path})
			recordSession(rec, dir)
			err = save.Run(context.Background())
		})

		require.NoError(t, err)
		assert.NotContains(t, stderr, "XDG_CONFIG_HOME")
		assert.FileExists(t, path)
	})
}
