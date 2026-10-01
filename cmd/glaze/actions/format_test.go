package actions

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
)

// buildFormat constructs an ActionFormat from the given profile contents and
// format-command flags, exercising the real parser and diagnostics manager.
func buildFormat(t *testing.T, profile string, flags map[string]string) *ActionFormat {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, ".glaze")
	assert.NoError(t, os.WriteFile(path, []byte(profile), 0o600))

	var (
		action *ActionFormat
		actErr error
	)

	cmd := &cli.Command{
		Name: "format",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "stdout"},
			&cli.BoolFlag{Name: "validate"},
			&cli.StringFlag{Name: "profile-path"},
			&cli.StringSliceFlag{Name: "var"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			action, actErr = NewFormat(c, "critical")
			return nil
		},
	}

	args := []string{"format", "--profile-path", path}
	for k, v := range flags {
		args = append(args, "--"+k, v)
	}

	assert.NoError(t, cmd.Run(context.Background(), args))
	assert.NoError(t, actErr)

	return action
}

func TestActionFormatRun(t *testing.T) {
	t.Run("rewrites the profile in place", func(t *testing.T) {
		messy := "session   {\n  name=\"demo\"\n}\n"
		action := buildFormat(t, messy, nil)

		assert.NoError(t, action.Run(context.Background()))

		contents, err := os.ReadFile(action.ProfilePath)
		assert.NoError(t, err)
		assert.Contains(t, string(contents), `name = "demo"`)
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

// captureStdout returns what fn writes to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create a pipe: %v", err)
	}

	previous := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = previous }()

	fn()
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("could not read stdout: %v", err)
	}

	return string(out)
}
