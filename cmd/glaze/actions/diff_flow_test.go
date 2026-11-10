package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// diffProfile starts its session in the directory of the profile, so a test knows every directory that diff compares.
const diffProfile = `session {
  name               = "demo"
  starting_directory = "."

  window {
    name  = "main"
    focus = true

    pane {
      name = "shell"
    }

    pane {
      name  = "logs"
      focus = true
    }
  }

  window {
    name = "run"

    pane {
      name = "server"
    }
  }
}
`

// readOnlyCommands are the tmux commands that diff may run; each one reads and none changes a session.
var readOnlyCommands = map[string]bool{"-V": true, "has-session": true, "ls": true, "lsw": true, "lsp": true, "display-message": true}

// liveWindowReplies holds the replies of one live window: its lsw line, its automatic-rename value and its lsp lines.
type liveWindowReplies struct {
	line      string
	automatic string
	panes     string
}

// buildDiff writes the profile into a temporary directory and builds ActionDiff for --profile-path with these flags.
// It returns the action, the recorder, the buffer that receives the report and the directory of the profile.
func buildDiff(t *testing.T, profile string, flags map[string]string) (*ActionDiff, *tmuxtest.Recorder, *bytes.Buffer, string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, ".glaze")
	require.NoError(t, os.WriteFile(path, []byte(profile), 0o600))

	diff, rec, out := buildDiffWith(t, append([]string{"--profile-path", path}, flagArgs(flags)...)...)

	return diff, rec, out, dir
}

// buildDiffByName writes the profile into a new profile registry and builds ActionDiff for its name, as `glaze diff <name>` does.
func buildDiffByName(t *testing.T, name, profile string, flags map[string]string) (*ActionDiff, *tmuxtest.Recorder, *bytes.Buffer) {
	t.Helper()

	writeRegistryProfile(t, newRegistry(t), name, profile)

	return buildDiffWith(t, append([]string{name}, flagArgs(flags)...)...)
}

// buildDiffWith builds ActionDiff from the arguments of diff, with a recorder for tmux. It skips the test when tmux is not installed.
// TMUX_PANE is cleared, so a test that runs inside tmux does not read the pane of the developer.
func buildDiffWith(t *testing.T, args ...string) (*ActionDiff, *tmuxtest.Recorder, *bytes.Buffer) {
	t.Helper()

	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")

	rec := tmuxtest.New().Default(tmuxtest.Result{})
	rec.Install(t)

	var (
		action *ActionDiff
		actErr error
	)

	cmd := &cli.Command{
		Name: "diff",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}},
			&cli.BoolFlag{Name: "debug"},
			&cli.StringFlag{Name: "socket-path"},
			&cli.StringFlag{Name: "socket-name"},
			&cli.StringFlag{Name: "profile-path"},
			&cli.StringFlag{Name: "var-file"},
			&cli.StringSliceFlag{Name: "var"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			action, actErr = NewDiff(c, "critical")
			return nil
		},
	}

	require.NoError(t, cmd.Run(context.Background(), append([]string{"diff"}, args...)))
	if errors.Is(actErr, tmux.ErrUnreachable) {
		t.Skipf("could not construct ActionDiff (tmux likely unavailable): %v", actErr)
	}

	require.NoError(t, actErr)

	var out bytes.Buffer
	action.out = &out

	return action, rec, &out
}

// recordLogs replaces the logger of the action with one that records every level into the returned buffer.
func recordLogs(diff *ActionDiff) *bytes.Buffer {
	var logs bytes.Buffer
	diff.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))}

	return &logs
}

// liveDemo queues the replies for the running session demo in dir: the version, the lookup, the host name and then each window.
func liveDemo(rec *tmuxtest.Recorder, dir string, windows ...liveWindowReplies) {
	rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
	rec.On("has-session", tmuxtest.Result{})
	rec.On("ls", tmuxtest.Result{Output: "$1;demo;" + dir})
	rec.On("display-message", tmuxtest.Result{Output: "buildhost"}) // #{host}

	lines := make([]string, 0, len(windows))
	for _, w := range windows {
		lines = append(lines, w.line)
	}

	rec.On("lsw", tmuxtest.Result{Output: strings.Join(lines, "\n")})
	for _, w := range windows {
		rec.On("display-message", tmuxtest.Result{Output: w.automatic}) // #{automatic-rename}
		rec.On("lsp", tmuxtest.Result{Output: w.panes})
	}
}

// matchingDemo queues a live session that matches diffProfile in dir.
func matchingDemo(rec *tmuxtest.Recorder, dir string) {
	liveDemo(rec, dir,
		liveWindowReplies{"@1;1;main;bb62,80x24,0,0;1", "0", "%1;1;shell;0;" + dir + "\n%2;2;logs;1;" + dir},
		liveWindowReplies{"@2;2;run;bb62,80x24,0,0;0", "0", "%3;1;server;1;" + dir},
	)
}

// notRunning queues the replies for a server without the session.
func notRunning(rec *tmuxtest.Recorder) {
	rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
	rec.On("has-session", tmuxtest.Failure("can't find session: demo"))
}

// assertReadOnly fails when diff ran a tmux command that is not a read, and when it ran a command that changes a session.
func assertReadOnly(t *testing.T, rec *tmuxtest.Recorder) {
	t.Helper()

	for _, call := range rec.Calls {
		assert.True(t, readOnlyCommands[call[0]], "diff ran `%s`, which is not a read", strings.Join(call, " "))
	}

	for _, forbidden := range []string{"new", "new-session", "neww", "new-window", "splitw", "split-window", "kill-session", "killw", "kill-window", "killp", "kill-pane", "send-keys", "selectw", "select-window", "selectp", "select-pane", "selectl", "select-layout", "set-option", "set", "setenv", "set-environment", "set-hook", "renamew", "rename-window", "resizep", "resize-pane", "attach", "switchc", "load-buffer", "wait-for"} {
		assert.False(t, rec.Called(forbidden), "diff ran `%s`", forbidden)
	}
}

// assertUnchangedProfile fails when the directory of the profile holds anything but the profile with its contents.
func assertUnchangedProfile(t *testing.T, dir, profile string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, ".glaze", entries[0].Name())

	contents, err := os.ReadFile(filepath.Join(dir, ".glaze")) //nolint:gosec // G304
	require.NoError(t, err)
	assert.Equal(t, profile, string(contents))
}

// noChangesReport is the whole report of a session that matches its profile.
func noChangesReport(name string) string {
	return fmt.Sprintf(noChangesFormat, `"`+name+`"`) + "\n" + notComparedProvenance + "\n" + notComparedGeometry + "\n"
}

func TestActionDiffRun(t *testing.T) {
	t.Run("returns nil and says so when the session matches the profile", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		matchingDemo(rec, dir)

		assert.NoError(t, diff.Run(context.Background()))

		assert.Equal(t, noChangesReport("demo"), out.String())
		assertReadOnly(t, rec)
		assertUnchangedProfile(t, dir, diffProfile)
		assert.Equal(t, []string{"-V", "has-session", "ls", "display-message", "lsw", "display-message", "lsp", "display-message", "lsp"}, subcommands(rec))
	})

	t.Run("reports a renamed window and an extra pane and returns the sentinel", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		liveDemo(rec, dir,
			liveWindowReplies{"@1;1;main;bb62,80x24,0,0;1", "0", "%1;1;shell;0;" + dir + "\n%2;2;logs;1;" + dir + "\n%4;3;buildhost;0;" + dir},
			liveWindowReplies{"@2;2;renamed;bb62,80x24,0,0;0", "0", "%3;1;server;1;" + dir},
		)

		err := diff.Run(context.Background())
		assert.ErrorIs(t, err, ErrDifferences)

		assert.Equal(t, strings.Join([]string{
			`session "demo"`,
			`  window "main" [1]`,
			`    - pane ` + labelUnnamed + ` [3]`,
			`  + window "run" [2]`,
			`    + pane "server" [1]  ` + dir,
			`  - window "renamed" [2]`,
			`    - pane "server" [1]`,
			"Plan: 2 to add, 0 to change, 3 to remove.",
			notComparedProvenance,
			notComparedGeometry,
			hintApply,
		}, "\n")+"\n", out.String())
		assertReadOnly(t, rec)
		assertUnchangedProfile(t, dir, diffProfile)
	})

	t.Run("reports a pane that glaze did not create as a change", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		liveDemo(rec, dir,
			liveWindowReplies{"@1;1;main;bb62,80x24,0,0;1", "0", "%1;1;buildhost;0;" + dir + "\n%2;2;logs;1;" + dir},
			liveWindowReplies{"@2;2;run;bb62,80x24,0,0;0", "0", "%3;1;server;1;" + dir},
		)

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)

		assert.Contains(t, out.String(), "    pane \"shell\" [1]\n      ~ title: "+labelUnnamed+" -> \"shell\"; "+reasonReplaced+"\n")
		assert.Contains(t, out.String(), "Plan: 0 to add, 1 to change, 0 to remove.\n")
		assert.True(t, strings.HasSuffix(out.String(), hintApply+"\n"))
		assertReadOnly(t, rec)
	})

	t.Run("returns nil for questions alone and counts them to check", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		sub := filepath.Join(dir, "sub")
		require.NoError(t, os.Mkdir(sub, 0o700))

		// A cd in shell and a window selected by hand are facts that tmux cannot attribute to glaze or to a user.
		liveDemo(rec, dir,
			liveWindowReplies{"@1;1;main;bb62,80x24,0,0;0", "0", "%1;1;shell;0;" + sub + "\n%2;2;logs;1;" + dir},
			liveWindowReplies{"@2;2;run;bb62,80x24,0,0;1", "0", "%3;1;server;1;" + dir},
		)

		assert.NoError(t, diff.Run(context.Background()))

		assert.Equal(t, strings.Join([]string{
			`session "demo"`,
			`  ? focus: window "run" [2], profile window "main" [1]; ` + reasonFocusWindow,
			`  window "main" [1]`,
			`    pane "shell" [1]`,
			`      ? directory: "` + sub + `", profile "` + dir + `"; ` + reasonDirectory,
			"Plan: 0 to add, 0 to change, 0 to remove, 2 to check.",
			notComparedProvenance,
			notComparedGeometry,
		}, "\n")+"\n", out.String())
		assertReadOnly(t, rec)
	})

	t.Run("matches a window under automatic-rename with the live window that tmux named", func(t *testing.T) {
		profile := strings.Replace(diffProfile, "    name  = \"main\"\n", "    name  = \"main\"\n    options = {\n      \"automatic-rename\" = \"on\"\n    }\n", 1)
		diff, rec, out, dir := buildDiff(t, profile, nil)
		liveDemo(rec, dir,
			liveWindowReplies{"@1;1;zsh;bb62,80x24,0,0;1", "1", "%1;1;shell;0;" + dir + "\n%2;2;logs;1;" + dir},
			liveWindowReplies{"@2;2;run;bb62,80x24,0,0;0", "0", "%3;1;server;1;" + dir},
		)

		assert.NoError(t, diff.Run(context.Background()))
		assert.Equal(t, noChangesReport("demo"), out.String())
		assertReadOnly(t, rec)
	})

	t.Run("prints a creation plan and returns the sentinel when the session does not run", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		notRunning(rec)

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)

		assert.Equal(t, strings.Join([]string{
			`+ session "demo"  ` + dir,
			`  + window "main" [1]`,
			`    + pane "shell" [1]  ` + dir,
			`    + pane "logs" [2]  ` + dir,
			`  + window "run" [2]`,
			`    + pane "server" [1]  ` + dir,
			"Plan: 6 to add, 0 to change, 0 to remove.",
			notComparedProvenance,
			notComparedGeometry,
			hintCreate,
		}, "\n")+"\n", out.String())
		assert.Equal(t, []string{"-V", "has-session"}, subcommands(rec))
		assert.Contains(t, rec.ArgsFor("has-session"), "=demo")
		assertUnchangedProfile(t, dir, diffProfile)
	})

	t.Run("prints a creation plan when no server runs", func(t *testing.T) {
		diff, rec, out, _ := buildDiff(t, validProfile, nil)
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
		rec.On("has-session", tmuxtest.Failure("no server running on /tmp/tmux-501/glaze-test"))

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)

		// Without a session directory the profile inherits the current directory, which the plan names as such.
		assert.Equal(t, strings.Join([]string{
			`+ session "demo"  ` + labelCurrentDir,
			`  + window "main" [1]`,
			`    + pane "shell" [1]  ` + labelCurrentDir,
			"Plan: 3 to add, 0 to change, 0 to remove.",
			notComparedProvenance,
			notComparedGeometry,
			hintCreate,
		}, "\n")+"\n", out.String())
		assert.False(t, rec.Called("ls"))
		assert.False(t, rec.Called("lsw"))
	})

	t.Run("says that a name from path depends on the current directory when the session does not run", func(t *testing.T) {
		project := filepath.Join(t.TempDir(), "proj")
		require.NoError(t, os.Mkdir(project, 0o700))
		t.Chdir(project)

		diff, rec, out := buildDiffByName(t, "reg", templateProfile, nil)
		logs := recordLogs(diff)
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
		rec.On("has-session", tmuxtest.Failure("can't find session: reg-proj"))

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)

		assert.Contains(t, rec.ArgsFor("has-session"), "=reg-proj")
		assert.Contains(t, out.String(), `+ session "reg-proj"  `+labelCurrentDir+"\n")
		assert.Contains(t, logs.String(), `level=INFO msg="the session name depends on the current directory; run diff from the directory that up ran in" session=reg-proj`)
	})

	t.Run("does not mention the current directory for a running session named from path", func(t *testing.T) {
		project := filepath.Join(t.TempDir(), "proj")
		require.NoError(t, os.Mkdir(project, 0o700))
		t.Chdir(project)

		diff, rec, _ := buildDiffByName(t, "reg", templateProfile, nil)
		logs := recordLogs(diff)
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;reg-proj;" + project})
		rec.On("display-message", tmuxtest.Result{Output: "buildhost"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1"})
		rec.On("display-message", tmuxtest.Result{Output: "0"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;shell;1;" + project})

		assert.NoError(t, diff.Run(context.Background()))
		assert.NotContains(t, logs.String(), "depends on the current directory")
	})

	t.Run("returns the unreachable sentinel and prints nothing", func(t *testing.T) {
		diff, rec, out, _ := buildDiff(t, diffProfile, nil)
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
		rec.On("has-session", tmuxtest.Failure("error connecting to /tmp/tmux-0/default (Permission denied)"))

		err := diff.Run(context.Background())
		assert.ErrorIs(t, err, tmux.ErrUnreachable)
		assert.NotErrorIs(t, err, ErrDifferences)
		assert.Empty(t, out.String())
	})

	t.Run("refuses an old tmux before any other call", func(t *testing.T) {
		diff, rec, out, _ := buildDiff(t, diffProfile, nil)
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.1b"})
		rec.On("has-session", tmuxtest.Result{})

		err := diff.Run(context.Background())
		assert.ErrorIs(t, err, tmux.ErrOldTmux)
		assert.NotErrorIs(t, err, ErrDifferences)
		assert.Empty(t, out.String())
		assert.Equal(t, []string{"-V"}, subcommands(rec))
	})

	t.Run("fails and prints nothing when a window is gone while glaze reads", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;" + dir})
		rec.On("display-message", tmuxtest.Result{Output: "buildhost"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;bb62,80x24,0,0;1\n@2;2;run;bb62,80x24,0,0;0"})
		rec.On("display-message", tmuxtest.Result{Output: "0"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;shell;0;" + dir + "\n%2;2;logs;1;" + dir})
		rec.On("display-message", tmuxtest.Result{Output: "0"})
		rec.On("lsp", tmuxtest.Failure("can't find window: @2"))

		err := diff.Run(context.Background())
		assert.ErrorContains(t, err, "the session changed while glaze read it; run the command again")
		assert.NotErrorIs(t, err, ErrDifferences)
		assert.Empty(t, out.String())
		assertReadOnly(t, rec)
	})

	t.Run("fails when the session is gone between the lookup and the listing", func(t *testing.T) {
		diff, rec, out, _ := buildDiff(t, diffProfile, nil)
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$2;other;/srv"})

		err := diff.Run(context.Background())
		assert.ErrorContains(t, err, "could not find session `demo`")
		assert.Empty(t, out.String())
	})

	t.Run("prints no directory line for a pane path that tmux did not report", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		liveDemo(rec, dir,
			liveWindowReplies{"@1;1;main;bb62,80x24,0,0;1", "0", "%1;1;shell;0;\n%2;2;logs;1;" + dir},
			liveWindowReplies{"@2;2;run;bb62,80x24,0,0;0", "0", "%3;1;server;1;" + dir},
		)

		assert.NoError(t, diff.Run(context.Background()))
		assert.Equal(t, noChangesReport("demo"), out.String())
	})

	t.Run("prints the inside hint when glaze runs in a pane of the compared session", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
		t.Setenv("TMUX_PANE", "%2")
		liveDemo(rec, dir,
			liveWindowReplies{"@1;1;main;bb62,80x24,0,0;1", "0", "%1;1;shell;0;" + dir + "\n%2;2;logs;1;" + dir},
			liveWindowReplies{"@2;2;renamed;bb62,80x24,0,0;0", "0", "%3;1;server;1;" + dir},
		)
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"}) // #{socket_path}
		rec.On("display-message", tmuxtest.Result{Output: "$1;demo;" + dir})        // the session of the pane

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)

		assert.True(t, strings.HasSuffix(out.String(), hintApplyInside+"\n"), out.String())
		assert.NotContains(t, out.String(), hintApply+"\n")
		assertReadOnly(t, rec)
	})

	t.Run("prints the plain hint when glaze runs in a pane of another session", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
		t.Setenv("TMUX_PANE", "%9")
		liveDemo(rec, dir,
			liveWindowReplies{"@1;1;main;bb62,80x24,0,0;1", "0", "%1;1;shell;0;" + dir + "\n%2;2;logs;1;" + dir},
			liveWindowReplies{"@2;2;renamed;bb62,80x24,0,0;0", "0", "%3;1;server;1;" + dir},
		)
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Output: "$2;other;/srv"})

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)

		assert.True(t, strings.HasSuffix(out.String(), hintApply+"\n"), out.String())
		assertReadOnly(t, rec)
	})

	t.Run("prints the plain hint inside another tmux server", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, nil)
		t.Setenv("TMUX", "/tmp/tmux-501/work,1234,0")
		t.Setenv("TMUX_PANE", "%2")
		liveDemo(rec, dir,
			liveWindowReplies{"@1;1;main;bb62,80x24,0,0;1", "0", "%1;1;shell;0;" + dir + "\n%2;2;logs;1;" + dir},
			liveWindowReplies{"@2;2;renamed;bb62,80x24,0,0;0", "0", "%3;1;server;1;" + dir},
		)
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)
		assert.True(t, strings.HasSuffix(out.String(), hintApply+"\n"), out.String())
	})

	t.Run("prints nothing with --quiet and still returns the sentinel", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, map[string]string{"quiet": "true"})
		liveDemo(rec, dir,
			liveWindowReplies{"@1;1;renamed;bb62,80x24,0,0;1", "0", "%1;1;shell;0;" + dir + "\n%2;2;logs;1;" + dir},
			liveWindowReplies{"@2;2;run;bb62,80x24,0,0;0", "0", "%3;1;server;1;" + dir},
		)

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)
		assert.Empty(t, out.String())
		assertReadOnly(t, rec)
	})

	t.Run("prints nothing with --quiet when the session matches", func(t *testing.T) {
		diff, rec, out, dir := buildDiff(t, diffProfile, map[string]string{"quiet": "true"})
		matchingDemo(rec, dir)

		assert.NoError(t, diff.Run(context.Background()))
		assert.Empty(t, out.String())
	})

	t.Run("warns about a session name that tmux rewrites and looks the sanitized name up", func(t *testing.T) {
		diff, rec, out, _ := buildDiff(t, strings.Replace(validProfile, `name = "demo"`, `name = "a.b:c"`, 1), nil)
		logs := recordLogs(diff)
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
		rec.On("has-session", tmuxtest.Failure("can't find session: a-b-c"))

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)

		assert.Contains(t, rec.ArgsFor("has-session"), "=a-b-c")
		assert.True(t, strings.HasPrefix(out.String(), `+ session "a-b-c"  `), out.String())
		assert.Contains(t, logs.String(), `level=WARN msg="tmux cannot use some characters in this session name; replacing them with hyphens" name=a.b:c tmux_name=a-b-c`)
	})

	t.Run("returns the diagnostics sentinel for an invalid profile before any tmux call", func(t *testing.T) {
		bad := strings.Replace(validProfile, `layout = "tiled"`, `layout = "noop"`, 1)
		diff, rec, out, _ := buildDiff(t, bad, nil)
		diags := captureDiagnostics(&diff.ActionBase)

		assert.ErrorIs(t, diff.Run(context.Background()), diagnostics.ErrHasDiagnostics)
		assert.Contains(t, diags.String(), "layout")
		assert.Empty(t, out.String())
		assert.Empty(t, rec.Calls)
	})

	t.Run("requires every variable before any tmux call", func(t *testing.T) {
		profile := "variable \"env\" {}\n\nsession {\n  name = \"demo-${var.env}\"\n\n  window {\n    name = \"main\"\n\n    pane {\n      name = \"shell\"\n    }\n  }\n}\n"
		diff, rec, out, _ := buildDiff(t, profile, nil)
		diags := captureDiagnostics(&diff.ActionBase)

		assert.ErrorIs(t, diff.Run(context.Background()), diagnostics.ErrHasDiagnostics)
		assert.Contains(t, diags.String(), "env")
		assert.Empty(t, out.String())
		assert.Empty(t, rec.Calls)
	})

	t.Run("resolves a variable from --var", func(t *testing.T) {
		profile := "variable \"env\" {}\n\nsession {\n  name = \"demo-${var.env}\"\n\n  window {\n    name = \"main\"\n\n    pane {\n      name = \"shell\"\n    }\n  }\n}\n"
		diff, rec, out, _ := buildDiff(t, profile, map[string]string{"var": "env=prod"})
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
		rec.On("has-session", tmuxtest.Failure("can't find session: demo-prod"))

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)
		assert.Contains(t, rec.ArgsFor("has-session"), "=demo-prod")
		assert.True(t, strings.HasPrefix(out.String(), `+ session "demo-prod"  `), out.String())
	})
}

func TestActionDiffByName(t *testing.T) {
	t.Run("compares the session that the registry profile names", func(t *testing.T) {
		diff, rec, out := buildDiffByName(t, "work", registryProfile, nil)
		registry := diff.profileDir()
		rec.On("-V", tmuxtest.Result{Output: "tmux 3.5a"})
		rec.On("has-session", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;" + registry})
		rec.On("display-message", tmuxtest.Result{Output: "buildhost"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1"})
		rec.On("display-message", tmuxtest.Result{Output: "0"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;shell;1;" + registry})

		assert.NoError(t, diff.Run(context.Background()))

		// The session is named after the profile, not after the registry name, and a relative directory is relative to the registry.
		assert.Equal(t, "work", diff.ProfileName)
		assert.Contains(t, rec.ArgsFor("has-session"), "=demo")
		assert.Equal(t, noChangesReport("demo"), out.String())
		for _, call := range rec.Calls {
			assert.NotContains(t, call, "=work")
		}
	})

	t.Run("rejects a registry profile without a session name before any tmux call", func(t *testing.T) {
		diff, rec, out := buildDiffByName(t, "work", noNameProfile, nil)
		diags := captureDiagnostics(&diff.ActionBase)

		assert.ErrorIs(t, diff.Run(context.Background()), diagnostics.ErrHasDiagnostics)

		assert.Contains(t, diags.String(), "Error: Missing session name")
		assert.Contains(t, diags.String(), "The profile `work` is in the profile registry, so its session must have a")
		assert.Empty(t, out.String())
		assert.Empty(t, rec.Calls, "the check must come before tmux")
	})

	t.Run("compares a path profile without a session name as default", func(t *testing.T) {
		diff, rec, out, _ := buildDiff(t, noNameProfile, nil)
		notRunning(rec)

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)
		assert.Contains(t, rec.ArgsFor("has-session"), "=default")
		assert.True(t, strings.HasPrefix(out.String(), `+ session "default"  `+labelCurrentDir+"\n"), out.String())
	})

	t.Run("fails before any tmux call from a deleted directory when the profile has no starting directory", func(t *testing.T) {
		writeRegistryProfile(t, newRegistry(t), "work", validProfile)
		inDeletedDirectory(t)

		diff, rec, out := buildDiffWith(t, "work")

		err := diff.Run(context.Background())
		assert.ErrorContains(t, err, "the session has no starting_directory, and glaze cannot read the current directory")
		assert.NotErrorIs(t, err, ErrDifferences)
		assert.Empty(t, out.String())
		assert.Empty(t, rec.Calls)
	})

	t.Run("runs from a deleted directory when the profile has a starting directory", func(t *testing.T) {
		writeRegistryProfile(t, newRegistry(t), "work", registryProfile)
		inDeletedDirectory(t)

		diff, rec, out := buildDiffWith(t, "work")
		notRunning(rec)

		assert.ErrorIs(t, diff.Run(context.Background()), ErrDifferences)
		assert.True(t, strings.HasPrefix(out.String(), `+ session "demo"  `+diff.profileDir()+"\n"), out.String())
	})
}
