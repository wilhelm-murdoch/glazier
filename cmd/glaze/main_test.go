package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"no error", nil, exitOK},
		{"a tmux command fails", errors.New("could not set option"), exitFailure},
		{"a usage error", fmt.Errorf("%w: unknown flag", errUsage), exitUsage},
		{"an invalid profile", fmt.Errorf("could not decode: %w", diagnostics.ErrHasDiagnostics), exitInvalidProfile},
		{"a missing profile", fmt.Errorf("%w: `x.glaze`", files.ErrProfileNotFound), exitInvalidProfile},
		{"tmux is unreachable", fmt.Errorf("%w: permission denied", tmux.ErrUnreachable), exitUnreachable},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, exitCode(c.err))
		})
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.glaze")
	assert.NoError(t, os.WriteFile(bad, []byte("session {\n  window {\n    layout = \"foo\"\n    pane {}\n  }\n}\n"), 0o600))

	cases := []struct {
		name   string
		args   []string
		want   int
		stderr string
	}{
		{"unknown flag", []string{"up", "--nope"}, exitUsage, "see `glaze up --help`"},
		{"unknown subcommand", []string{"nope"}, exitUsage, "nope"},
		{"malformed --var", []string{"format", "--var", "nokey", "--profile-path", bad}, exitUsage, "key=value"},
		{"unknown log level", []string{"--log-level", "loud", "format"}, exitUsage, "loud"},
		{"missing profile", []string{"format", "--profile-path", filepath.Join(dir, "missing.glaze")}, exitInvalidProfile, "glaze profile not found"},
		{"invalid profile", []string{"format", "--validate", "--profile-path", bad}, exitInvalidProfile, "contains errors"},
		{"version", []string{"--version"}, exitOK, ""},
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
}
