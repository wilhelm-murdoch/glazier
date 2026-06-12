package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "profile.glaze")
	assert.NoError(t, os.WriteFile(file, []byte("session {}"), 0o600))

	t.Run("returns true for an existing file", func(t *testing.T) {
		assert.True(t, FileExists(file))
	})

	t.Run("returns false for a missing file", func(t *testing.T) {
		assert.False(t, FileExists(filepath.Join(dir, "nope.glaze")))
	})

	t.Run("returns false for a directory", func(t *testing.T) {
		assert.False(t, FileExists(dir))
	})
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	assert.NoError(t, err)

	t.Run("expands a leading tilde to the home directory", func(t *testing.T) {
		assert.Equal(t, filepath.Join(home, "foo"), ExpandPath("~/foo"))
	})

	t.Run("leaves absolute paths untouched", func(t *testing.T) {
		assert.Equal(t, "/etc/hosts", ExpandPath("/etc/hosts"))
	})

	t.Run("leaves a bare tilde untouched", func(t *testing.T) {
		assert.Equal(t, "~root", ExpandPath("~root"))
	})

	t.Run("leaves relative paths untouched", func(t *testing.T) {
		assert.Equal(t, "foo/bar", ExpandPath("foo/bar"))
	})
}

func TestResolveProfilePath(t *testing.T) {
	t.Run("returns the explicit profile path when it exists", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "explicit.glaze")
		assert.NoError(t, os.WriteFile(file, []byte("session {}"), 0o600))

		resolved, err := ResolveProfilePath(file)
		assert.NoError(t, err)
		assert.Equal(t, file, resolved)
	})

	t.Run("errors when the explicit profile path does not exist", func(t *testing.T) {
		resolved, err := ResolveProfilePath("/definitely/not/here.glaze")
		assert.Error(t, err)
		assert.Equal(t, "/definitely/not/here.glaze", resolved)
		assert.Contains(t, err.Error(), "could not locate profile")
	})

	t.Run("finds .glaze in the current working directory", func(t *testing.T) {
		dir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(dir, ".glaze"), []byte("session {}"), 0o600))

		chdir(t, dir)
		t.Setenv("GLAZE_PATH", "")

		cwd, err := os.Getwd()
		assert.NoError(t, err)

		resolved, err := ResolveProfilePath("")
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(cwd, ".glaze"), resolved)
	})

	t.Run("falls back to GLAZE_PATH when no local profile exists", func(t *testing.T) {
		cwd := t.TempDir()
		glazeDir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(glazeDir, ".glaze"), []byte("session {}"), 0o600))

		chdir(t, cwd)
		t.Setenv("GLAZE_PATH", glazeDir)

		resolved, err := ResolveProfilePath("")
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(glazeDir, ".glaze"), resolved)
	})

	t.Run("errors when no profile can be located", func(t *testing.T) {
		cwd := t.TempDir()
		chdir(t, cwd)
		t.Setenv("GLAZE_PATH", "")

		_, err := ResolveProfilePath("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "glaze profile not found")
	})
}

// chdir changes into dir for the duration of the test and restores the previous
// working directory afterwards.
func chdir(t *testing.T, dir string) {
	t.Helper()

	prev, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})
}
