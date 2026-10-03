package cases

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// malformedFixtures holds profiles that are broken on purpose, so glaze cannot format them.
const malformedFixtures = "malformed"

// TestFixtures keeps every fixture in the canonical format, so that a fixture
// reads like a profile that a user writes and formats.
func TestFixtures(t *testing.T) {
	harness.Run(t, "canonical", func(c *harness.Case) {
		root := filepath.Join(c.Env().Root, "fixtures")
		fixtures := os.DirFS(root)
		err := fs.WalkDir(fixtures, ".", func(rel string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && rel == malformedFixtures {
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
			if c.OK(r, rel+" formats") {
				c.Equal(rel+" is in the canonical format", string(want), r.Stdout)
			}
			return nil
		})
		if err != nil {
			c.T().Fatal(err)
		}
	})
}
