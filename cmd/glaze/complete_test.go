package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// complete runs glaze in completion mode with the words after `glaze` and returns what reached the app writer and standard error.
// The run must return nil: a failing completer breaks every Tab.
func complete(t *testing.T, words ...string) (string, string) {
	t.Helper()

	var out, errOut bytes.Buffer
	app := newApp()
	app.Writer = &out
	app.ErrWriter = &errOut

	readStderr := capture(t, &os.Stderr)
	err := app.Run(context.Background(), slices.Concat([]string{"glaze"}, words, []string{"--generate-shell-completion"}))
	stderr := readStderr() + errOut.String()

	assert.NoError(t, err)

	return out.String(), stderr
}

func TestCompleteProfileNames(t *testing.T) {
	registry := registryHome(t)
	writeProfile(t, registry, "work")
	writeProfile(t, registry, "api")
	require.NoError(t, os.WriteFile(filepath.Join(registry, "notes.txt"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(registry, ".hidden.glaze"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(registry, "bad name.glaze"), []byte("x"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(registry, "sub"), 0o700))

	cases := []struct {
		name  string
		words []string
		want  string
	}{
		{"up lists the names", []string{"up"}, "api\nwork\n"},
		{"down lists the names", []string{"down"}, "api\nwork\n"},
		{"format lists the names", []string{"format"}, "api\nwork\n"},
		{"diff lists the names", []string{"diff"}, "api\nwork\n"},
		{"a given name ends the completion on diff", []string{"diff", "work"}, ""},
		{"--profile-path ends the completion on diff", []string{"diff", "--profile-path", "x"}, ""},
		{"a given name ends the completion", []string{"up", "work"}, ""},
		{"--profile-path ends the completion", []string{"up", "--profile-path", "x"}, ""},
		{"--session ends the completion", []string{"down", "--session", "x"}, ""},
		{"save lists no name", []string{"save"}, ""},
		{"ls lists no name", []string{"ls"}, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, stderr := complete(t, c.words...)
			assert.Equal(t, c.want, out)
			assert.Empty(t, stderr)
		})
	}

	t.Run("a flag prefix completes the flags", func(t *testing.T) {
		out, stderr := complete(t, "up", "--p")
		assert.Contains(t, out, "--profile-path:")
		assert.NotContains(t, out, "work")
		assert.Empty(t, stderr)
	})

	t.Run("a flag prefix on diff completes the quiet flag", func(t *testing.T) {
		// The prefix has two letters, because the flag package reads `--q` as the alias `-q` itself.
		out, stderr := complete(t, "diff", "--qu")
		assert.Contains(t, out, "--quiet:")
		assert.NotContains(t, out, "work")
		assert.Empty(t, stderr)
	})

	t.Run("the root lists diff among the commands", func(t *testing.T) {
		// The library reads os.Args for the root completion, so the test sets what a shell sends.
		saved := os.Args
		os.Args = []string{"glaze", "--generate-shell-completion"}
		t.Cleanup(func() { os.Args = saved })

		out, stderr := complete(t)
		assert.Contains(t, out, "diff:")
		assert.Contains(t, out, "up:")
		assert.NotContains(t, out, "work")
		assert.Empty(t, stderr)
	})

	t.Run("a missing registry prints nothing", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		out, stderr := complete(t, "up")
		assert.Empty(t, out)
		assert.Empty(t, stderr)
	})

	t.Run("an unreadable registry prints nothing", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads every directory")
		}

		unreadable := registryHome(t)
		writeProfile(t, unreadable, "work")
		require.NoError(t, os.Chmod(unreadable, 0))
		t.Cleanup(func() { _ = os.Chmod(unreadable, 0o700) }) //nolint:gosec // G302: the test directory gets its mode back for the clean-up

		out, stderr := complete(t, "up")
		assert.Empty(t, out)
		assert.Empty(t, stderr)
	})
}
