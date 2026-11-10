package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// registryHome points HOME at a temporary directory with an empty profile registry and returns the registry.
// The three variables that could move the registry or add a profile are cleared, so the host environment never leaks in.
func registryHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GLAZE_REGISTRY", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("GLAZE_PATH", "")
	t.Setenv("NO_COLOR", "1")

	registry := filepath.Join(home, ".config", "glaze")
	require.NoError(t, os.MkdirAll(registry, 0o700))

	return registry
}

// writeProfile puts a profile with the session name name into the registry and returns its path.
func writeProfile(t *testing.T, registry, name string) string {
	t.Helper()

	path := filepath.Join(registry, name+files.ProfileSuffix)
	require.NoError(t, os.WriteFile(path, []byte("session {\n  name = \""+name+"\"\n}\n"), 0o600))

	return path
}

// capture replaces the file that target points to, os.Stdout or os.Stderr, with a pipe.
// The returned function restores the file and returns what was written; the clean-up does the same, so a failed require cannot block the reader.
func capture(t *testing.T, target **os.File) func() string {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	saved := *target
	*target = writer

	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- string(data)
	}()

	// The first call closes the writer and drains the reader, and every later call gets the same text.
	var once sync.Once
	var text string
	read := func() string {
		once.Do(func() {
			*target = saved
			require.NoError(t, writer.Close())
			text = <-done
			require.NoError(t, reader.Close())
		})

		return text
	}

	t.Cleanup(func() { read() })

	return read
}

// requireTmux skips a case that builds a tmux client, because NewDown looks up the binary before it reads the profile.
func requireTmux(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not on PATH")
	}
}

// assertInOrder checks that each line is in text and comes after the line before it.
func assertInOrder(t *testing.T, text string, lines ...string) {
	t.Helper()

	last := -1
	for _, line := range lines {
		at := strings.Index(text, line)
		assert.Greater(t, at, last, "`%s` is missing or out of order in:\n%s", line, text)
		last = at
	}
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"no error", nil, exitOK},
		{"a tmux command fails", errors.New("could not set option"), exitFailure},
		{"a tmux command exits non-zero", fmt.Errorf("could not set option: %w", tmux.NewCommandErrorWithOutput([]string{"tmux"}, exec.Command("false").Run(), "bad value: maybe")), exitFailure},
		{"tmux exits non-zero and is unreachable", fmt.Errorf("%w: %w", tmux.ErrUnreachable, tmux.NewCommandErrorWithOutput([]string{"tmux"}, exec.Command("false").Run(), "Permission denied")), exitUnreachable},
		{"a usage error", fmt.Errorf("%w: unknown flag", errUsage), exitUsage},
		{"an invalid profile name", fmt.Errorf("%w: a/b", files.ErrInvalidProfileName), exitUsage},
		{"a registry that cannot be used", fmt.Errorf("%w: GLAZE_REGISTRY must be an absolute path", files.ErrRegistryUnusable), exitInvalidProfile},
		{"an invalid profile", fmt.Errorf("could not decode: %w", diagnostics.ErrHasDiagnostics), exitInvalidProfile},
		{"a missing profile", fmt.Errorf("%w: `x.glaze`", files.ErrProfileNotFound), exitInvalidProfile},
		{"tmux is unreachable", fmt.Errorf("%w: permission denied", tmux.ErrUnreachable), exitUnreachable},
		{"tmux is too old", fmt.Errorf("%w: found tmux 3.0a", tmux.ErrOldTmux), exitUnreachable},
		{"stopped by SIGINT", fmt.Errorf("%w: exit status 1", signalError{signal: syscall.SIGINT}), 130},
		{"stopped by SIGTERM", fmt.Errorf("%w: %w", signalError{signal: syscall.SIGTERM}, tmux.ErrUnreachable), 143},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, exitCode(c.err))
		})
	}
}

func TestCancelOnSignal(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	stop := cancelOnSignal(cancel)
	t.Cleanup(stop)

	assert.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case <-ctx.Done():
		assert.Equal(t, signalError{signal: syscall.SIGTERM}, context.Cause(ctx))
		assert.EqualError(t, context.Cause(ctx), "glaze stopped on SIGTERM")
	case <-time.After(5 * time.Second):
		t.Fatal("SIGTERM did not cancel the run")
	}
}

func TestRunNamesTheSignal(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(signalError{signal: syscall.SIGINT})

	var stderr bytes.Buffer
	assert.Equal(t, 130, run(ctx, []string{"glaze", "ls", "--socket-name", "glaze-test-no-server"}, &stderr))
	assert.Contains(t, stderr.String(), "glaze stopped on SIGINT")
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.glaze")
	assert.NoError(t, os.WriteFile(bad, []byte("session {\n  window {\n    layout = \"foo\"\n    pane {}\n  }\n}\n"), 0o600))

	registry := registryHome(t)
	writeProfile(t, registry, "work")

	cases := []struct {
		name   string
		args   []string
		want   int
		stderr string
	}{
		{"unknown flag", []string{"up", "--nope"}, exitUsage, "see `glaze up --help`"},
		{"unknown subcommand", []string{"nope"}, exitUsage, "unknown command `nope`"},
		{"no arguments shows the help", nil, exitOK, ""},
		{"malformed --var", []string{"format", "--var", "nokey", "--profile-path", bad}, exitUsage, "key=value"},
		{"unknown log level", []string{"--log-level", "loud", "format"}, exitUsage, "`loud` is not a log level; use one of critical, debug, error, info, trace, warning"},
		{"missing profile", []string{"format", "--profile-path", filepath.Join(dir, "missing.glaze")}, exitInvalidProfile, "glaze profile not found"},
		{"invalid profile", []string{"format", "--validate", "--profile-path", bad}, exitInvalidProfile, "contains errors"},
		{"version flag after a subcommand", []string{"up", "-v"}, exitUsage, "flag provided but not defined: -v"},
		{"long version flag after a subcommand", []string{"down", "--version"}, exitUsage, "flag provided but not defined: -version"},
		{"completion script", []string{"completion", "zsh"}, exitOK, ""},
		{"completion without a shell shows the help", []string{"completion"}, exitOK, ""},
		{"completion for an unknown shell", []string{"completion", "nope"}, exitUsage, "unknown shell `nope`"},
		{"a second name", []string{"up", "a", "b"}, exitUsage, "`glaze up` takes one name; got `a` and `b`"},
		{"a name and --profile-path", []string{"up", "work", "--profile-path", "p"}, exitUsage, "give a name or --profile-path, not both"},
		{"--profile-path before a name", []string{"up", "--profile-path", "p", "work"}, exitUsage, "give a name or --profile-path, not both"},
		{"a name and --session", []string{"down", "work", "--session", "s"}, exitUsage, "give a name or --session, not both"},
		{"a name that starts with a dash", []string{"up", "-1", "--detached"}, exitUsage, "cannot start with `-`"},
		{"an empty name", []string{"up", ""}, exitUsage, "cannot be empty"},
		{"a name without a profile", []string{"format", "--validate", "nope"}, exitInvalidProfile, "does not exist"},
		// No leaf gets a help subcommand, so `help` is an ordinary profile name.
		{"the help word is a profile name", []string{"up", "help"}, exitInvalidProfile, "no profile named `help` in the profile registry"},
		{"the usage pointer of a bad name", []string{"format", "a/b"}, exitUsage, "(see `glaze format --help`)"},
		{"an unknown command that names a profile", []string{"work"}, exitUsage, "unknown command `work`; to apply the profile `work`, run `glaze up work`"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stderr bytes.Buffer
			assert.Equal(t, c.want, run(context.Background(), append([]string{"glaze"}, c.args...), &stderr))
			assert.Contains(t, stderr.String(), c.stderr)
		})
	}

	t.Run("tmux is not on PATH", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		var stderr bytes.Buffer
		assert.Equal(t, exitUnreachable, run(context.Background(), []string{"glaze", "ls"}, &stderr))
		assert.Contains(t, stderr.String(), "tmux is unreachable")
	})

	t.Run("an unknown command keeps the plain message", func(t *testing.T) {
		// No profile `nope` exists, and `a/b` is not a profile name, so the hint cannot apply.
		for _, word := range []string{"nope", "a/b"} {
			var stderr bytes.Buffer
			assert.Equal(t, exitUsage, run(context.Background(), []string{"glaze", word}, &stderr), word)
			assert.Equal(t, "usage error: unknown command `"+word+"` (see `glaze --help`)\n", stderr.String(), word)
		}
	})

	t.Run("an unknown command keeps the plain message when the registry is a file", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".config"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "glaze"), []byte("x"), 0o600))

		// The hint is best effort, so a registry that is a file gives none and the message stays plain.
		var stderr bytes.Buffer
		assert.Equal(t, exitUsage, run(context.Background(), []string{"glaze", "work"}, &stderr))
		assert.Equal(t, "usage error: unknown command `work` (see `glaze --help`)\n", stderr.String())
	})

	t.Run("a usage error comes before the tmux lookup", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		// The words are checked before the action builds its tmux client, so the usage error comes first.
		for _, c := range []struct {
			args    []string
			message string
		}{
			{[]string{"ls", "x"}, "`glaze ls` takes no argument; got `x` (see `glaze ls --help`)"},
			{[]string{"save", "a/b"}, "`a/b` is not a profile name: a name cannot contain `/`; for a file, use --profile-path (see `glaze save --help`)"},
			{[]string{"save", "work.glaze"}, "(see `glaze save --help`)"},
			{[]string{"save", "-1"}, "(see `glaze save --help`)"},
			{[]string{"save", "work", "--profile-path", "p"}, "give a name or --profile-path, not both (see `glaze save --help`)"},
		} {
			var stderr bytes.Buffer
			assert.Equal(t, exitUsage, run(context.Background(), append([]string{"glaze"}, c.args...), &stderr), strings.Join(c.args, " "))
			assert.Contains(t, stderr.String(), c.message, strings.Join(c.args, " "))
		}
	})

	t.Run("a name and --session on save reach the action", func(t *testing.T) {
		requireTmux(t)

		// On save the flag names the session and the name names the file, so checkArgs lets both through.
		var stderr bytes.Buffer
		assert.Equal(t, exitFailure, run(context.Background(), []string{"glaze", "save", "work", "--session", "sv", "--socket-name", "glaze-test-no-server"}, &stderr))
		assert.Contains(t, stderr.String(), "no tmux server is running")
		assert.NotContains(t, stderr.String(), "not both")
	})

	t.Run("a name without a registry says how save creates it", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		missing := filepath.Join(home, ".config", "glaze")

		var stderr bytes.Buffer
		assert.Equal(t, exitInvalidProfile, run(context.Background(), []string{"glaze", "up", "nope"}, &stderr))
		assert.Contains(t, stderr.String(), "no profile named `nope`; the profile registry `"+missing+"` does not exist")

		// The save pointer comes before the mkdir line, because it is the way that needs no hand work.
		assertInOrder(t, stderr.String(),
			" - `glaze save nope` in a tmux session creates the registry and writes the profile",
			" - or create it with `mkdir -p "+missing+"` and put `nope.glaze` in it",
			" - --profile-path reads a profile from any path",
		)
	})

	t.Run("a name without a profile in a registry says how save writes it", func(t *testing.T) {
		var stderr bytes.Buffer
		assert.Equal(t, exitInvalidProfile, run(context.Background(), []string{"glaze", "up", "nope"}, &stderr))

		// The ls line comes first, then the save pointer, then --profile-path.
		assertInOrder(t, stderr.String(),
			"no profile named `nope` in the profile registry; `"+filepath.Join(registry, "nope.glaze")+"` does not exist",
			" - `glaze ls --profiles` lists the profiles in `"+registry+"`",
			" - `glaze save nope` in a tmux session writes the profile",
			" - --profile-path reads a profile from any path",
		)
	})

	t.Run("the help names the registry", func(t *testing.T) {
		for _, c := range []struct {
			args    []string
			phrases []string
		}{
			{[]string{"up", "--help"}, []string{"glaze up [options] [name]", "profile registry"}},
			{[]string{"help", "up"}, []string{"glaze up [options] [name]", "profile registry"}},
			{[]string{"save", "--help"}, []string{"glaze save [options] [name]", "A name writes <name>.glaze into the profile registry", "replaces the target profile when it exists"}},
		} {
			var out bytes.Buffer
			app := newApp()
			app.Writer = &out
			assert.NoError(t, app.Run(context.Background(), append([]string{"glaze"}, c.args...)), strings.Join(c.args, " "))

			for _, phrase := range c.phrases {
				assert.Contains(t, out.String(), phrase, strings.Join(c.args, " "))
			}
		}
	})

	t.Run("the version line goes to the writer of the app", func(t *testing.T) {
		for _, flag := range []string{"--version", "-v"} {
			var out bytes.Buffer
			app := newApp()
			app.Writer = &out
			assert.NoError(t, app.Run(context.Background(), []string{"glaze", flag}), flag)
			assert.Equal(t, fmt.Sprintf("Version: %s, Stage: %s, Commit: %s, Date: %s\n", Version, Stage, Commit, Date), out.String(), flag)
		}
	})

	t.Run("format prints a registry profile without tmux", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		// The empty value is not a path, so the name and the flag do not conflict.
		for _, args := range [][]string{{"format", "--stdout", "work"}, {"format", "--stdout", "--profile-path=", "work"}} {
			readStdout := capture(t, &os.Stdout)
			var stderr bytes.Buffer
			code := run(context.Background(), append([]string{"glaze"}, args...), &stderr)
			assert.Equal(t, exitOK, code)
			assert.Equal(t, "session {\n  name = \"work\"\n}\n", readStdout())
			assert.Empty(t, stderr.String())
		}
	})

	t.Run("a registry profile without a session name is an error", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(registry, "noname.glaze"), []byte("session {\n  window {\n    pane {}\n  }\n}\n"), 0o600))

		// The diagnostics writer prints to os.Stderr; the run writer gets only the summary line.
		readStderr := capture(t, &os.Stderr)
		var stderr bytes.Buffer
		code := run(context.Background(), []string{"glaze", "format", "--validate", "noname"}, &stderr)
		diagnostic := readStderr()

		assert.Equal(t, exitInvalidProfile, code)
		assert.Contains(t, diagnostic, "Missing session name")
		assert.Contains(t, diagnostic, "`glaze down noname` could kill an unrelated session")
		assert.Contains(t, stderr.String(), "contains errors")
	})

	t.Run("down without the registry profile points at --session", func(t *testing.T) {
		requireTmux(t)

		var stderr bytes.Buffer
		assert.Equal(t, exitInvalidProfile, run(context.Background(), []string{"glaze", "down", "nope"}, &stderr))
		assert.Contains(t, stderr.String(), "no profile named `nope` in the profile registry")
		assert.Contains(t, stderr.String(), " - to kill a session by its tmux name, use --session")
	})

	t.Run("ls --profiles needs no tmux", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		// Only a regular `<name>.glaze` is listed, by name; a bad name warns, and the rest is skipped without a word.
		listed := registryHome(t)
		api := writeProfile(t, listed, "api")
		work := writeProfile(t, listed, "work")
		require.NoError(t, os.WriteFile(filepath.Join(listed, "bad name.glaze"), []byte("x"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(listed, ".hidden.glaze"), []byte("x"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(listed, "notes.txt"), []byte("x"), 0o600))
		require.NoError(t, os.Mkdir(filepath.Join(listed, "sub"), 0o700))

		readStdout := capture(t, &os.Stdout)
		readStderr := capture(t, &os.Stderr)
		var stderr bytes.Buffer
		code := run(context.Background(), []string{"glaze", "ls", "--profiles"}, &stderr)
		stdout, logged := readStdout(), readStderr()

		assert.Equal(t, exitOK, code)
		assert.Equal(t, "NAME  PATH\napi   "+api+"\nwork  "+work+"\n", stdout)
		assert.Equal(t, 1, strings.Count(logged, "WRN"), logged)
		assert.Contains(t, logged, "bad name.glaze")
		assert.Empty(t, stderr.String())
	})

	t.Run("ls --profiles without a registry", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		readStdout := capture(t, &os.Stdout)
		readStderr := capture(t, &os.Stderr)
		var stderr bytes.Buffer
		code := run(context.Background(), []string{"glaze", "ls", "--profiles"}, &stderr)
		stdout, logged := readStdout(), readStderr()

		assert.Equal(t, exitOK, code)
		assert.Empty(t, stdout)
		assert.Equal(t, 1, strings.Count(logged, "INF no profile registry; `glaze save <name>` creates it"), logged)
		assert.Empty(t, stderr.String())
	})

	t.Run("ls --profiles with a registry that is a file", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".config"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "glaze"), []byte("x"), 0o600))

		var stderr bytes.Buffer
		assert.Equal(t, exitInvalidProfile, run(context.Background(), []string{"glaze", "ls", "--profiles"}, &stderr))
		assert.Contains(t, stderr.String(), "is a file, not the profile registry directory")
	})

	t.Run("ls --profiles with an unreadable registry", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads every directory")
		}

		unreadable := registryHome(t)
		require.NoError(t, os.Chmod(unreadable, 0))
		t.Cleanup(func() { _ = os.Chmod(unreadable, 0o700) }) //nolint:gosec // G302: the test directory gets its mode back for the clean-up

		var stderr bytes.Buffer
		assert.Equal(t, exitInvalidProfile, run(context.Background(), []string{"glaze", "ls", "--profiles"}, &stderr))
		assert.Contains(t, stderr.String(), "permission denied")
	})

	t.Run("no name and no profile anywhere", func(t *testing.T) {
		t.Chdir(t.TempDir())

		var stderr bytes.Buffer
		assert.Equal(t, exitInvalidProfile, run(context.Background(), []string{"glaze", "format"}, &stderr))

		// The three lines keep their order, and the fourth names the registry.
		assertInOrder(t, stderr.String(),
			"tried using --profile-path",
			"searching the current directory",
			"looking up GLAZE_PATH environment variable",
			"no profile name was given; `glaze up <name>` reads the profile registry and `glaze ls --profiles` lists it",
		)
	})
}

func TestCheckArgsWithoutArgs(t *testing.T) {
	// A command built outside the library has no Args at all, so there is nothing to check.
	assert.NoError(t, checkArgs(&cli.Command{Name: "up"}, true))
	assert.NoError(t, checkArgs(&cli.Command{Name: "ls"}, false))
}
