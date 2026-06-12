package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

// runWithProfile runs NewActionBase against the given profile contents through a
// real cli.Command so the profile-path flag is populated as it would be in
// production, and returns the resulting base and error.
func runWithProfile(t *testing.T, contents string) (*ActionBase, error) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, ".glaze")
	assert.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	var (
		base    *ActionBase
		baseErr error
	)

	cmd := &cli.Command{
		Name: "test",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "profile-path"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			base, baseErr = NewActionBase(c, "critical")
			return nil
		},
	}

	assert.NoError(t, cmd.Run(context.Background(), []string{"test", "--profile-path", path}))

	return base, baseErr
}

func TestNewActionBase(t *testing.T) {
	t.Run("returns an error for a profile with a syntax error", func(t *testing.T) {
		base, err := runWithProfile(t, "session \"x\" {\n  window {\n")

		assert.Nil(t, base)
		assert.Error(t, err)
	})

	t.Run("succeeds for a valid profile", func(t *testing.T) {
		base, err := runWithProfile(t, "session {\n  name = \"demo\"\n}\n")

		assert.NoError(t, err)
		assert.NotNil(t, base)
	})
}
