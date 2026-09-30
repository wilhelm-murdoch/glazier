package actions

import (
	"bytes"
	"context"
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

		assert.NoError(t, action.Run())

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

		assert.ErrorIs(t, action.Run(), diagnostics.ErrHasDiagnostics)
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

		assert.NoError(t, action.Run())
		assert.Contains(t, out.String(), "Warning: Session name will be changed")

		contents, err := os.ReadFile(action.ProfilePath)
		assert.NoError(t, err)
		assert.Contains(t, string(contents), "    name = \"w\"")
	})

	t.Run("validation does not write warnings into --stdout output", func(t *testing.T) {
		action := buildFormat(t, "session {\n  name = \"a.b\"\n  window {\n    pane {}\n  }\n}\n", map[string]string{"validate": "true", "stdout": "true"})

		var out bytes.Buffer
		action.DiagnosticsManager.Writer = hcl.NewDiagnosticTextWriter(
			&out,
			map[string]*hcl.File{action.ProfilePath: action.Parser.File},
			0,
			false,
		)

		assert.NoError(t, action.Run())
		assert.Empty(t, out.String())
	})

	t.Run("validation passes for a valid profile", func(t *testing.T) {
		action := buildFormat(t, validProfile, map[string]string{"validate": "true", "stdout": "true"})
		assert.NoError(t, action.Run())
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
		assert.ErrorIs(t, action.Run(), diagnostics.ErrHasDiagnostics)
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
		assert.NoError(t, action.Run())
	})
}
