package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

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
		{"a tmux command exits non-zero", fmt.Errorf("could not set option: %w", tmux.NewCommandErrorWithOutput([]string{"tmux"}, exec.Command("false").Run(), "bad value: maybe")), exitFailure},
		{"tmux exits non-zero and is unreachable", fmt.Errorf("%w: %w", tmux.ErrUnreachable, tmux.NewCommandErrorWithOutput([]string{"tmux"}, exec.Command("false").Run(), "Permission denied")), exitUnreachable},
		{"a usage error", fmt.Errorf("%w: unknown flag", errUsage), exitUsage},
		{"an invalid profile", fmt.Errorf("could not decode: %w", diagnostics.ErrHasDiagnostics), exitInvalidProfile},
		{"a missing profile", fmt.Errorf("%w: `x.glaze`", files.ErrProfileNotFound), exitInvalidProfile},
		{"tmux is unreachable", fmt.Errorf("%w: permission denied", tmux.ErrUnreachable), exitUnreachable},
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
		{"unknown log level", []string{"--log-level", "loud", "format"}, exitUsage, "loud"},
		{"missing profile", []string{"format", "--profile-path", filepath.Join(dir, "missing.glaze")}, exitInvalidProfile, "glaze profile not found"},
		{"invalid profile", []string{"format", "--validate", "--profile-path", bad}, exitInvalidProfile, "contains errors"},
		{"version", []string{"--version"}, exitOK, ""},
		{"short version", []string{"-v"}, exitOK, ""},
		{"version flag after a subcommand", []string{"up", "-v"}, exitUsage, "flag provided but not defined: -v"},
		{"long version flag after a subcommand", []string{"down", "--version"}, exitUsage, "flag provided but not defined: -version"},
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
