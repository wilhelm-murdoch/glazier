package cases

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// malformedDir holds the profiles that are broken on purpose, so glaze cannot format them.
const malformedDir = "malformed"

// TestFixtures keeps every fixture in the canonical format, so that a fixture reads like a profile that a user writes and formats.
func TestFixtures(t *testing.T) {
	harness.Run(t, "canonical", func(c *harness.Case) {
		root := filepath.Join(c.Env().Root, "fixtures")
		fixtures := os.DirFS(root)
		err := fs.WalkDir(fixtures, ".", func(rel string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if d.IsDir() && rel == malformedDir {
				return fs.SkipDir
			}

			if d.IsDir() || !strings.HasSuffix(rel, ".glaze") {
				return nil
			}

			want, err := fs.ReadFile(fixtures, rel)
			if err != nil {
				return err
			}

			r := c.Glaze("format", "--stdout", "--profile-path", filepath.Join(root, rel))
			if c.OK(r, fmt.Sprintf("%s formats", rel)) {
				c.Equal(fmt.Sprintf("%s is in the canonical format", rel), string(want), r.Stdout)
			}

			return nil
		})

		if err != nil {
			c.T().Fatal(err)
		}
	})
}
