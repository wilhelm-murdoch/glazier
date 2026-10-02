package files

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// read returns the contents of path, or fails the test.
func read(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path) //nolint:gosec // G304: a test file.
	assert.NoError(t, err)

	return string(contents)
}

// entries returns the names in dir, so a test can see a temporary file that was left behind.
func entries(t *testing.T, dir string) []string {
	t.Helper()

	list, err := os.ReadDir(dir)
	assert.NoError(t, err)

	var names []string
	for _, entry := range list {
		names = append(names, entry.Name())
	}

	return names
}

func TestWriteFile(t *testing.T) {
	t.Run("creates a new file and leaves no temporary file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".glaze")

		assert.NoError(t, WriteFile(path, []byte("new"), 0o644))
		assert.Equal(t, "new", read(t, path))
		assert.Equal(t, []string{".glaze"}, entries(t, dir))
	})

	t.Run("replaces an existing file and keeps its mode", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".glaze")
		assert.NoError(t, os.WriteFile(path, []byte("old"), 0o600))

		assert.NoError(t, WriteFile(path, []byte("new"), 0o644))
		assert.Equal(t, "new", read(t, path))

		info, err := os.Stat(path)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		assert.Equal(t, []string{".glaze"}, entries(t, dir))
	})

	t.Run("writes through a symlink and keeps the link", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "real.glaze")
		link := filepath.Join(dir, ".glaze")
		assert.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
		assert.NoError(t, os.Symlink(target, link))

		assert.NoError(t, WriteFile(link, []byte("new"), 0o644))
		assert.Equal(t, "new", read(t, target))

		info, err := os.Lstat(link)
		assert.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink)
	})

	t.Run("refuses a symlink whose target does not exist", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), ".glaze")
		assert.NoError(t, os.Symlink(filepath.Join(filepath.Dir(link), "gone"), link))

		assert.ErrorContains(t, WriteFile(link, []byte("new"), 0o644), "could not resolve")
	})

	t.Run("refuses a pipe and a directory", func(t *testing.T) {
		dir := t.TempDir()
		fifo := filepath.Join(dir, "p.glaze")
		assert.NoError(t, syscall.Mkfifo(fifo, 0o600))

		// An open of a FIFO with no reader blocks, so the test fails after a timeout when WriteFile opens it.
		done := make(chan error, 1)
		go func() { done <- WriteFile(fifo, []byte("new"), 0o644) }()

		select {
		case err := <-done:
			assert.ErrorIs(t, err, ErrNotRegular)
		case <-time.After(5 * time.Second):
			t.Fatal("WriteFile blocked on the FIFO")
		}

		assert.ErrorIs(t, WriteFile(dir, []byte("new"), 0o644), ErrNotRegular)
	})

	t.Run("refuses a read-only file and leaves it as it is", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write a read-only file")
		}

		dir := t.TempDir()
		path := filepath.Join(dir, ".glaze")
		assert.NoError(t, os.WriteFile(path, []byte("old"), 0o400))

		assert.ErrorIs(t, WriteFile(path, []byte("new"), 0o644), os.ErrPermission)
		assert.Equal(t, "old", read(t, path))
		assert.Equal(t, []string{".glaze"}, entries(t, dir))
	})

	t.Run("keeps the old file when the write fails", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write to a read-only directory")
		}

		dir := t.TempDir()
		path := filepath.Join(dir, ".glaze")
		assert.NoError(t, os.WriteFile(path, []byte("old"), 0o600))

		// The directory refuses the temporary file, so the write fails before it touches the profile.
		assert.NoError(t, os.Chmod(dir, 0o500))        //nolint:gosec // G302: a read-only test directory.
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restores a test directory.

		assert.Error(t, WriteFile(path, []byte("new"), 0o644))
		assert.Equal(t, "old", read(t, path))
	})
}

func TestIsRegular(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	fifo := filepath.Join(dir, "p")
	link := filepath.Join(dir, "l")
	assert.NoError(t, os.WriteFile(file, nil, 0o600))
	assert.NoError(t, syscall.Mkfifo(fifo, 0o600))
	assert.NoError(t, os.Symlink(file, link))

	assert.True(t, IsRegular(file))
	assert.True(t, IsRegular(link))
	assert.False(t, IsRegular(fifo))
	assert.False(t, IsRegular(dir))
	assert.False(t, IsRegular(filepath.Join(dir, "missing")))
}
