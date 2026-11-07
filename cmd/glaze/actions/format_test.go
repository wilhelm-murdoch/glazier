package actions

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
)

// formatResult is what one run of the format command gives: the error of the run itself and the result of NewFormat.
type formatResult struct {
	action *ActionFormat
	runErr error
	err    error
}

// buildFormat constructs an ActionFormat from the given profile contents and
// format-command flags, exercising the real parser and diagnostics manager.
func buildFormat(t *testing.T, profile string, flags map[string]string) *ActionFormat {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, ".glaze")
	assert.NoError(t, os.WriteFile(path, []byte(profile), 0o600))

	action, err := newFormatAt(t, path, flags)
	assert.NoError(t, err)

	return action
}

// newFormatAt runs NewFormat for the profile at path, as main does, and returns its result.
func newFormatAt(t *testing.T, path string, flags map[string]string) (*ActionFormat, error) {
	t.Helper()

	return newFormatWith(t, append([]string{"--profile-path", path}, flagArgs(flags)...)...)
}

// newFormatWith runs NewFormat with the arguments of format, as main does, and returns its result.
func newFormatWith(t *testing.T, args ...string) (*ActionFormat, error) {
	t.Helper()

	result := runNewFormat(args...)
	assert.NoError(t, result.runErr)

	return result.action, result.err
}

// newFormatWithTimeout is newFormatWith for a profile that can block the read, such as a FIFO: it fails the test when NewFormat does not return in five seconds.
// The goroutine never touches t, because a call on t after the test fails on the timeout makes the test binary panic.
func newFormatWithTimeout(t *testing.T, args ...string) (*ActionFormat, error) {
	t.Helper()

	done := make(chan formatResult, 1)
	go func() { done <- runNewFormat(args...) }()

	var result formatResult
	select {
	case result = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("NewFormat blocked on the profile")
	}

	assert.NoError(t, result.runErr)

	return result.action, result.err
}

// runNewFormat runs NewFormat with the arguments of format through a real cli.Command. It uses no t, so a goroutine can call it.
func runNewFormat(args ...string) formatResult {
	var result formatResult

	cmd := &cli.Command{
		Name: "format",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "stdout"},
			&cli.BoolFlag{Name: "validate"},
			&cli.StringFlag{Name: "profile-path"},
			&cli.StringSliceFlag{Name: "var"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			result.action, result.err = NewFormat(c, "critical")
			return nil
		},
	}

	result.runErr = cmd.Run(context.Background(), append([]string{"format"}, args...))

	return result
}

func TestNewActionBaseShowsTheSourceOfASyntaxError(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".glaze")
	assert.NoError(t, os.WriteFile(path, []byte("session {\n  window { pane {} name = \"x\"\n  }\n}\n"), 0o600))

	var err error
	stderr := capture(t, &os.Stderr, func() { _, err = newFormatAt(t, path, nil) })

	assert.ErrorIs(t, err, diagnostics.ErrHasDiagnostics)
	assert.Contains(t, stderr, `window { pane {} name = "x"`)
	assert.NotContains(t, stderr, "source code not available")
}

func TestActionFormatRun(t *testing.T) {
	t.Run("leaves a formatted profile untouched", func(t *testing.T) {
		action := buildFormat(t, validProfile, nil)

		old := time.Now().Add(-time.Hour).Truncate(time.Second)
		assert.NoError(t, os.Chtimes(action.ProfilePath, old, old))
		before, err := os.Stat(action.ProfilePath)
		assert.NoError(t, err)

		assert.NoError(t, action.Run(context.Background()))

		after, err := os.Stat(action.ProfilePath)
		assert.NoError(t, err)
		assert.Equal(t, old, after.ModTime())
		assert.True(t, os.SameFile(before, after), "the file must not be replaced")
	})

	t.Run("refuses to format a pipe in place, before it reads it", func(t *testing.T) {
		fifo := filepath.Join(t.TempDir(), "p.glaze")
		assert.NoError(t, syscall.Mkfifo(fifo, 0o600))

		// A read of a FIFO with no writer blocks, so the test fails on the timeout when the refusal comes after the read.
		_, err := newFormatWithTimeout(t, "--profile-path", fifo)

		assert.ErrorIs(t, err, files.ErrNotRegular)
		assert.ErrorContains(t, err, "use --stdout")
	})

	t.Run("rewrites the profile in place", func(t *testing.T) {
		messy := "session   {\n  name=\"demo\"\n}\n"
		action := buildFormat(t, messy, nil)

		assert.NoError(t, action.Run(context.Background()))

		contents, err := os.ReadFile(action.ProfilePath)
		assert.NoError(t, err)
		assert.Contains(t, string(contents), `name = "demo"`)
	})

	t.Run("reports a failed write as a failure, not as an invalid profile", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write to a read-only directory")
		}

		action := buildFormat(t, "session   {\n  name=\"demo\"\n}\n", nil)
		dir := filepath.Dir(action.ProfilePath)
		assert.NoError(t, os.Chmod(dir, 0o555))        //nolint:gosec // G302: the test directory is read-only on purpose
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) //nolint:gosec // G302: the test directory gets its mode back for the clean-up

		err := action.Run(context.Background())
		assert.ErrorContains(t, err, "could not write")
		assert.NotErrorIs(t, err, diagnostics.ErrHasDiagnostics)
	})

	t.Run("validates and reports an invalid layout", func(t *testing.T) {
		bad := `session {
  name = "demo"
  window {
    name   = "main"
    layout = "noop"
    pane { name = "shell" }
  }
}
`
		action := buildFormat(t, bad, map[string]string{"validate": "true"})

		assert.ErrorIs(t, action.Run(context.Background()), diagnostics.ErrHasDiagnostics)
	})

	t.Run("validation shows a warning and still formats the profile", func(t *testing.T) {
		unformatted := "session {\n  name = \"a.b\"\n  window {\n  name = \"w\"\n    pane {}\n  }\n}\n"
		action := buildFormat(t, unformatted, map[string]string{"validate": "true"})

		var out bytes.Buffer
		action.DiagnosticsManager.Writer = hcl.NewDiagnosticTextWriter(
			&out,
			map[string]*hcl.File{action.ProfilePath: action.Parser.File},
			0,
			false,
		)

		assert.NoError(t, action.Run(context.Background()))
		assert.Contains(t, out.String(), "Warning: Session name will be changed")

		contents, err := os.ReadFile(action.ProfilePath)
		assert.NoError(t, err)
		assert.Contains(t, string(contents), "    name = \"w\"")
	})

	t.Run("validation with --stdout keeps warnings out of the formatted output", func(t *testing.T) {
		profile := "session {\n  name = \"a.b\"\n  window {\n    pane {}\n  }\n}\n"
		action := buildFormat(t, profile, map[string]string{"validate": "true", "stdout": "true"})

		var diags bytes.Buffer
		action.DiagnosticsManager.Writer = hcl.NewDiagnosticTextWriter(
			&diags,
			map[string]*hcl.File{action.ProfilePath: action.Parser.File},
			0,
			false,
		)

		stdout := captureStdout(t, func() {
			assert.NoError(t, action.Run(context.Background()))
		})

		assert.Equal(t, profile, stdout)
		assert.Contains(t, diags.String(), "Warning: Session name will be changed")
	})

	t.Run("validation passes for a valid profile", func(t *testing.T) {
		action := buildFormat(t, validProfile, map[string]string{"validate": "true", "stdout": "true"})
		assert.NoError(t, action.Run(context.Background()))
	})

	t.Run("validation enforces a required variable", func(t *testing.T) {
		profile := `variable "region" {
  type = string
}

session {
  name = "svc-${var.region}"

  window {
    pane {}
  }
}
`
		action := buildFormat(t, profile, map[string]string{"validate": "true", "stdout": "true"})
		assert.ErrorIs(t, action.Run(context.Background()), diagnostics.ErrHasDiagnostics)
	})

	t.Run("validation passes when the required variable is supplied", func(t *testing.T) {
		profile := `variable "region" {
  type = string
}

session {
  name = "svc-${var.region}"

  window {
    pane {}
  }
}
`
		action := buildFormat(t, profile, map[string]string{
			"validate": "true", "stdout": "true", "var": "region=us-east-1",
		})

		assert.NoError(t, action.Run(context.Background()))
	})
}

func TestActionFormatByName(t *testing.T) {
	const messy = "session   {\n  name=\"demo\"\n}\n"
	const formatted = "session {\n  name = \"demo\"\n}\n"

	t.Run("rewrites the registry profile in place through a symlink and keeps its mode", func(t *testing.T) {
		registry := newRegistry(t)
		assert.NoError(t, os.MkdirAll(registry, 0o700))

		target := filepath.Join(t.TempDir(), "work.glaze")
		assert.NoError(t, os.WriteFile(target, []byte(messy), 0o600))
		link := filepath.Join(registry, "work.glaze")
		assert.NoError(t, os.Symlink(target, link))

		action, err := newFormatWith(t, "work")
		assert.NoError(t, err)
		assert.Equal(t, link, action.ProfilePath)
		assert.Equal(t, "work", action.ProfileName)

		assert.NoError(t, action.Run(context.Background()))

		info, err := os.Lstat(link)
		assert.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink, "the link must stay")

		contents, err := os.ReadFile(target) //nolint:gosec // G304: a test file.
		assert.NoError(t, err)
		assert.Equal(t, formatted, string(contents))

		info, err = os.Stat(target)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("prints the registry profile with --stdout and leaves the file", func(t *testing.T) {
		path := writeRegistryProfile(t, newRegistry(t), "work", messy)

		action, err := newFormatWith(t, "--stdout", "work")
		assert.NoError(t, err)

		stdout := captureStdout(t, func() {
			assert.NoError(t, action.Run(context.Background()))
		})

		assert.Equal(t, formatted, stdout)

		contents, err := os.ReadFile(path) //nolint:gosec // G304: a test file.
		assert.NoError(t, err)
		assert.Equal(t, messy, string(contents))
	})

	t.Run("reports a diagnostic of a registry profile with the registry path", func(t *testing.T) {
		bad := `session {
  name = "demo"
  window {
    name   = "main"
    layout = "noop"
    pane { name = "shell" }
  }
}
`
		path := writeRegistryProfile(t, newRegistry(t), "work", bad)

		action, err := newFormatWith(t, "--validate", "work")
		assert.NoError(t, err)

		var diags bytes.Buffer
		action.DiagnosticsManager.Writer = hcl.NewDiagnosticTextWriter(
			&diags,
			map[string]*hcl.File{action.ProfilePath: action.Parser.File},
			0,
			false,
		)

		assert.ErrorIs(t, action.Run(context.Background()), diagnostics.ErrHasDiagnostics)
		assert.Contains(t, diags.String(), "on "+path+" line 5")
		assert.Contains(t, diags.String(), `layout = "noop"`)
	})

	t.Run("rejects a registry profile without a session name with --validate", func(t *testing.T) {
		writeRegistryProfile(t, newRegistry(t), "work", noNameProfile)

		action, err := newFormatWith(t, "--validate", "--stdout", "work")
		assert.NoError(t, err)
		diags := captureDiagnostics(&action.ActionBase)

		stdout := captureStdout(t, func() {
			assert.ErrorIs(t, action.Run(context.Background()), diagnostics.ErrHasDiagnostics)
		})

		assert.Empty(t, stdout)
		assert.Contains(t, diags.String(), "Error: Missing session name")
		assert.Contains(t, diags.String(), "`glaze down work` could kill an unrelated session")
	})

	t.Run("formats a registry profile without a session name when it does not validate", func(t *testing.T) {
		path := writeRegistryProfile(t, newRegistry(t), "work", "session   {\n  window {\n    pane {}\n  }\n}\n")

		action, err := newFormatWith(t, "work")
		assert.NoError(t, err)
		assert.NoError(t, action.Run(context.Background()))

		contents, err := os.ReadFile(path) //nolint:gosec // G304: a test file.
		assert.NoError(t, err)
		assert.Equal(t, "session {\n  window {\n    pane {}\n  }\n}\n", string(contents))
	})

	t.Run("validates a path profile without a session name", func(t *testing.T) {
		action := buildFormat(t, noNameProfile, map[string]string{"validate": "true", "stdout": "true"})
		diags := captureDiagnostics(&action.ActionBase)

		stdout := captureStdout(t, func() {
			assert.NoError(t, action.Run(context.Background()))
		})

		assert.Equal(t, noNameProfile, stdout)
		assert.Empty(t, diags.String())
	})

	t.Run("refuses to format a pipe in the registry in place, before it reads it", func(t *testing.T) {
		registry := newRegistry(t)
		assert.NoError(t, os.MkdirAll(registry, 0o700))
		fifo := filepath.Join(registry, "f.glaze")
		assert.NoError(t, syscall.Mkfifo(fifo, 0o600))

		// A read of a FIFO with no writer blocks, so the test fails on the timeout when the refusal comes after the read.
		_, err := newFormatWithTimeout(t, "f")

		assert.ErrorIs(t, err, files.ErrNotRegular)
		assert.ErrorContains(t, err, "could not format `"+fifo+"` in place")
		assert.ErrorContains(t, err, "use --stdout")
	})
}

// captureStdout returns what fn writes to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	return capture(t, &os.Stdout, fn)
}

// capture returns what fn writes to stream, which is &os.Stdout or &os.Stderr.
func capture(t *testing.T, stream **os.File, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create a pipe: %v", err)
	}

	previous := *stream
	*stream = w
	defer func() { *stream = previous }()

	fn()
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("could not read the output: %v", err)
	}

	return string(out)
}
