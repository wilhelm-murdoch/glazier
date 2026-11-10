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

// buildLs constructs a fully wired ActionLs with a tmuxtest recorder installed
// and its output captured into the returned buffer.
func buildLs(t *testing.T) (*ActionLs, *tmuxtest.Recorder, *bytes.Buffer) {
	t.Helper()

	action, rec, out, err := newLsWith(t)
	if err != nil {
		t.Skipf("could not construct ActionLs (tmux likely unavailable): %v", err)
	}

	return action, rec, out
}

// buildLsProfiles constructs ActionLs for `ls --profiles` without tmux on PATH, because the listing of the registry must not need it.
// The recorder shows that no tmux command runs.
func buildLsProfiles(t *testing.T) (*ActionLs, *tmuxtest.Recorder, *bytes.Buffer) {
	t.Helper()

	t.Setenv("PATH", t.TempDir())

	action, rec, out, err := newLsWith(t, "--profiles")
	require.NoError(t, err)

	return action, rec, out
}

// newLsWith runs NewLs with the arguments of ls, with a recorder installed, and returns its result with a buffer for its output.
func newLsWith(t *testing.T, args ...string) (*ActionLs, *tmuxtest.Recorder, *bytes.Buffer, error) {
	t.Helper()

	rec := tmuxtest.New().Default(tmuxtest.Result{})
	rec.Install(t)

	var (
		action *ActionLs
		actErr error
	)

	cmd := &cli.Command{
		Name: "ls",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "profiles"},
			&cli.StringFlag{Name: "socket-path"},
			&cli.StringFlag{Name: "socket-name"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			action, actErr = NewLs(c, "critical")
			return nil
		},
	}

	assert.NoError(t, cmd.Run(context.Background(), append([]string{"ls"}, args...)))
	if actErr != nil {
		return nil, rec, nil, actErr
	}

	out := &bytes.Buffer{}
	action.out = out

	return action, rec, out, nil
}

// quietLogs sends the log lines of the action to a buffer, so a test can read them.
func quietLogs(ls *ActionLs) *bytes.Buffer {
	var logs bytes.Buffer
	ls.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

	return &logs
}

func TestActionLsRun(t *testing.T) {
	t.Run("renders a table of sessions with window counts", func(t *testing.T) {
		t.Setenv("TMUX", "")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp\n$2;gig-watson;/home/v"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;main;tiled;1\n@2;logs;tiled;0"})
		rec.On("lsw", tmuxtest.Result{Output: "@3;main;tiled;1"})

		assert.NoError(t, ls.Run(context.Background()))

		rendered := out.String()
		assert.Contains(t, rendered, "NAME")
		assert.Contains(t, rendered, "demo")
		assert.Contains(t, rendered, "gig-watson")
		assert.Contains(t, rendered, "/home/v")
		// demo has two windows, gig-watson has one.
		assert.Regexp(t, `demo\s+2\s+/tmp`, rendered)
		assert.Regexp(t, `gig-watson\s+1\s+/home/v`, rendered)
	})

	t.Run("marks the session of the pane that glaze runs in", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
		t.Setenv("TMUX_PANE", "%4")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp\n$2;other;/srv"})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Output: "$2;other;/srv"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;main;tiled;1"})
		rec.On("lsw", tmuxtest.Result{Output: "@2;main;tiled;1"})

		assert.NoError(t, ls.Run(context.Background()))

		assert.Regexp(t, `other\*\s+1\s+/srv`, out.String())
		assert.NotRegexp(t, `demo\*`, out.String())
	})

	t.Run("prints nothing and succeeds when no tmux server is running", func(t *testing.T) {
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Failure("no server running on /tmp/tmux-1000/default"))

		var logs bytes.Buffer
		ls.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		assert.NoError(t, ls.Run(context.Background()))
		assert.Empty(t, out.String())
		assert.Contains(t, logs.String(), "no tmux server is running")
		assert.False(t, rec.Called("ls"))
	})

	t.Run("prints only the header for a server with no sessions", func(t *testing.T) {
		t.Setenv("TMUX", "")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: ""})

		assert.NoError(t, ls.Run(context.Background()))
		assert.Equal(t, "NAME  WINDOWS  PATH\n", out.String())
	})

	t.Run("errors when tmux is unreachable", func(t *testing.T) {
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Failure("error connecting to /tmp/tmux-0/default (Permission denied)"))

		assert.ErrorIs(t, ls.Run(context.Background()), tmux.ErrUnreachable)
		assert.Empty(t, out.String())
	})

	t.Run("propagates window listing failures", func(t *testing.T) {
		t.Setenv("TMUX", "")
		ls, rec, _ := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("lsw", tmuxtest.Result{Err: assert.AnError})

		err := ls.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not list windows")
	})

	t.Run("marks no session inside another tmux server", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
		t.Setenv("TMUX_PANE", "%4")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp\n$2;other;/srv"})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/work"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;main;tiled;1"})
		rec.On("lsw", tmuxtest.Result{Output: "@2;main;tiled;1"})

		assert.NoError(t, ls.Run(context.Background()))
		assert.NotContains(t, out.String(), "*")
		assert.Equal(t, 1, rec.CountOf("display-message"))
	})

	t.Run("errors when the current session cannot be determined", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
		t.Setenv("TMUX_PANE", "%4")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Err: assert.AnError})

		err := ls.Run(context.Background())
		assert.ErrorIs(t, err, assert.AnError)
		assert.Empty(t, out.String())
	})
}

func TestActionLsProfiles(t *testing.T) {
	t.Run("renders the name and the path of each profile in name order without tmux", func(t *testing.T) {
		registry := newRegistry(t)
		work := writeRegistryProfile(t, registry, "work", validProfile)
		api := writeRegistryProfile(t, registry, "api", validProfile)
		ls, rec, out := buildLsProfiles(t)

		assert.NoError(t, ls.Run(context.Background()))

		assert.Equal(t, "NAME  PATH\napi   "+api+"\nwork  "+work+"\n", out.String())
		assert.Empty(t, rec.Calls, "the registry listing must not call tmux")
	})

	t.Run("renders only the header for an empty registry", func(t *testing.T) {
		registry := newRegistry(t)
		assert.NoError(t, os.MkdirAll(registry, 0o700))
		ls, rec, out := buildLsProfiles(t)

		assert.NoError(t, ls.Run(context.Background()))

		assert.Equal(t, "NAME  PATH\n", out.String())
		assert.Empty(t, rec.Calls)
	})

	t.Run("prints nothing for a missing registry and says so", func(t *testing.T) {
		registry := newRegistry(t)
		ls, rec, out := buildLsProfiles(t)
		logs := quietLogs(ls)

		assert.NoError(t, ls.Run(context.Background()))

		assert.Empty(t, out.String())
		assert.Contains(t, logs.String(), "level=INFO")
		assert.Contains(t, logs.String(), "no profile registry")
		assert.Contains(t, logs.String(), "path="+registry)
		assert.Empty(t, rec.Calls)
	})

	t.Run("errors when the registry is a file", func(t *testing.T) {
		registry := newRegistry(t)
		assert.NoError(t, os.MkdirAll(filepath.Dir(registry), 0o700))
		assert.NoError(t, os.WriteFile(registry, []byte("not a directory"), 0o600))
		ls, _, out := buildLsProfiles(t)

		err := ls.Run(context.Background())

		assert.ErrorIs(t, err, files.ErrRegistryUnusable)
		assert.ErrorContains(t, err, "is a file, not the profile registry directory; move it away, or set XDG_CONFIG_HOME or GLAZE_REGISTRY to a directory")
		assert.Empty(t, out.String())
	})

	t.Run("sorts by name, not by file name", func(t *testing.T) {
		registry := newRegistry(t)
		ab := writeRegistryProfile(t, registry, "a-b", validProfile)
		a := writeRegistryProfile(t, registry, "a", validProfile)
		ls, _, out := buildLsProfiles(t)

		assert.NoError(t, ls.Run(context.Background()))

		// The file `a-b.glaze` sorts before `a.glaze`, but the name `a` comes first.
		assert.Equal(t, "NAME  PATH\na     "+a+"\na-b   "+ab+"\n", out.String())
	})

	t.Run("reads GLAZE_REGISTRY and ignores XDG_CONFIG_HOME without a warning", func(t *testing.T) {
		newRegistry(t)
		registry := filepath.Join(t.TempDir(), "reg")
		work := writeRegistryProfile(t, registry, "work", validProfile)
		t.Setenv("GLAZE_REGISTRY", registry)
		t.Setenv("XDG_CONFIG_HOME", "rel")
		ls, _, out := buildLsProfiles(t)
		logs := quietLogs(ls)

		assert.NoError(t, ls.Run(context.Background()))

		assert.Equal(t, "NAME  PATH\nwork  "+work+"\n", out.String())
		assert.Empty(t, logs.String())
	})

	t.Run("warns once when a relative XDG_CONFIG_HOME is ignored", func(t *testing.T) {
		registry := newRegistry(t)
		work := writeRegistryProfile(t, registry, "work", validProfile)
		t.Setenv("XDG_CONFIG_HOME", "rel")
		ls, _, out := buildLsProfiles(t)
		logs := quietLogs(ls)

		assert.NoError(t, ls.Run(context.Background()))

		assert.Equal(t, "NAME  PATH\nwork  "+work+"\n", out.String())
		assert.Equal(t, 1, strings.Count(logs.String(), "XDG_CONFIG_HOME is not absolute, so the profile registry is ~/.config/glaze"))
		assert.Contains(t, logs.String(), "level=WARN")
		assert.Contains(t, logs.String(), "XDG_CONFIG_HOME=rel")
	})

	t.Run("errors when the registry cannot be read", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can read a directory without permissions")
		}

		registry := newRegistry(t)
		assert.NoError(t, os.MkdirAll(registry, 0o700))
		assert.NoError(t, os.Chmod(registry, 0o000))
		t.Cleanup(func() { _ = os.Chmod(registry, 0o700) }) //nolint:gosec // G302: the test directory gets its mode back for the clean-up
		ls, _, out := buildLsProfiles(t)

		err := ls.Run(context.Background())

		assert.ErrorIs(t, err, files.ErrRegistryUnusable)
		assert.ErrorContains(t, err, "could not read `"+registry+"`")
		assert.ErrorContains(t, err, "permission denied")
		assert.Empty(t, out.String())
	})

	t.Run("warns once for an entry that is not a profile name and lists the others", func(t *testing.T) {
		registry := newRegistry(t)
		work := writeRegistryProfile(t, registry, "work", validProfile)
		bad := filepath.Join(registry, "bad name.glaze")
		assert.NoError(t, os.WriteFile(bad, []byte(validProfile), 0o600))
		ls, _, out := buildLsProfiles(t)
		logs := quietLogs(ls)

		assert.NoError(t, ls.Run(context.Background()))

		assert.Equal(t, "NAME  PATH\nwork  "+work+"\n", out.String())
		assert.Equal(t, 1, strings.Count(logs.String(), "registry entry is not a profile"))
		assert.Contains(t, logs.String(), "level=WARN")
		assert.Contains(t, logs.String(), bad)
		assert.Contains(t, logs.String(), "a name can contain only ASCII letters, digits, `.`, `_` and `-`")
	})

	t.Run("warns once for a symlink whose target does not exist", func(t *testing.T) {
		registry := newRegistry(t)
		assert.NoError(t, os.MkdirAll(registry, 0o700))
		gone := filepath.Join(t.TempDir(), "gone.glaze")
		assert.NoError(t, os.Symlink(gone, filepath.Join(registry, "gone.glaze")))
		ls, _, out := buildLsProfiles(t)
		logs := quietLogs(ls)

		assert.NoError(t, ls.Run(context.Background()))

		assert.Equal(t, "NAME  PATH\n", out.String())
		assert.Equal(t, 1, strings.Count(logs.String(), "registry entry is not a profile"))
		assert.Contains(t, logs.String(), "a symlink to `"+gone+"`, which does not exist")
	})

	t.Run("skips a hidden file, a file without the suffix and a directory without a word", func(t *testing.T) {
		registry := newRegistry(t)
		work := writeRegistryProfile(t, registry, "work", validProfile)
		assert.NoError(t, os.WriteFile(filepath.Join(registry, ".hidden.glaze"), []byte(validProfile), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(registry, "notes.txt"), []byte("notes"), 0o600))
		assert.NoError(t, os.Mkdir(filepath.Join(registry, "sub.glaze"), 0o700))
		ls, _, out := buildLsProfiles(t)
		logs := quietLogs(ls)

		assert.NoError(t, ls.Run(context.Background()))

		assert.Equal(t, "NAME  PATH\nwork  "+work+"\n", out.String())
		assert.Empty(t, logs.String())
	})
}
