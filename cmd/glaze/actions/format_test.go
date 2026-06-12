package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

	t.Run("validation passes for a valid profile", func(t *testing.T) {
		action := buildFormat(t, validProfile, map[string]string{"validate": "true", "stdout": "true"})
		assert.NoError(t, action.Run())
	})
}
