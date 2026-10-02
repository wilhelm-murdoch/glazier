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
	home := t.TempDir()
	t.Setenv("HOME", home)

	for path, want := range map[string]string{
		"~":          home,
		"~/":         home + "/",
		"~/foo":      filepath.Join(home, "foo"),
		"/etc/hosts": "/etc/hosts",
		"foo/bar":    "foo/bar",
		"a/~/b":      "a/~/b",
		"":           "",
	} {
		got, err := ExpandPath(path)
		assert.NoError(t, err, path)
		assert.Equal(t, want, got, path)
	}

	t.Run("rejects ~user", func(t *testing.T) {
		_, err := ExpandPath("~root/x")
		assert.ErrorIs(t, err, ErrTildeUser)
	})

	t.Run("fails without a home directory", func(t *testing.T) {
		t.Setenv("HOME", "")

		_, err := ExpandPath("~/foo")
		assert.ErrorContains(t, err, "could not expand `~`")
	})
}

func TestResolveDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for path, want := range map[string]string{
		"/srv/app":   "/srv/app",
		"sub":        "/base/sub",
		"./sub/../x": "/base/x",
		"..":         "/",
		"~/proj":     filepath.Join(home, "proj"),
		"~":          home,
	} {
		got, err := ResolveDirectory(path, "/base")
		assert.NoError(t, err, path)
		assert.Equal(t, want, got, path)
	}

	_, err := ResolveDirectory("~root", "/base")
	assert.ErrorIs(t, err, ErrTildeUser)
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
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.Contains(t, err.Error(), "/definitely/not/here.glaze")
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

	t.Run("expands ~ in the explicit profile path", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		assert.NoError(t, os.WriteFile(filepath.Join(home, "p.glaze"), []byte("session {}"), 0o600))

		resolved, err := ResolveProfilePath("~/p.glaze")
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(home, "p.glaze"), resolved)
	})

	t.Run("rejects ~user in the explicit profile path", func(t *testing.T) {
		_, err := ResolveProfilePath("~root/p.glaze")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorIs(t, err, ErrTildeUser)
	})

	t.Run("expands ~ in GLAZE_PATH", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		assert.NoError(t, os.Mkdir(filepath.Join(home, "gp"), 0o700))
		assert.NoError(t, os.WriteFile(filepath.Join(home, "gp", ".glaze"), []byte("session {}"), 0o600))

		chdir(t, t.TempDir())
		t.Setenv("GLAZE_PATH", "~/gp")

		resolved, err := ResolveProfilePath("")
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(home, "gp", ".glaze"), resolved)
	})

	t.Run("errors when no profile can be located", func(t *testing.T) {
		cwd := t.TempDir()
		chdir(t, cwd)
		t.Setenv("GLAZE_PATH", "")

		_, err := ResolveProfilePath("")
		assert.ErrorIs(t, err, ErrProfileNotFound)
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

func TestResolveProfilePathSaysWhatIsWrong(t *testing.T) {
	t.Run("--profile-path is a directory", func(t *testing.T) {
		_, err := ResolveProfilePath(t.TempDir())
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "is a directory; --profile-path must name a profile file")
	})

	t.Run(".glaze in the current directory is a directory", func(t *testing.T) {
		cwd := t.TempDir()
		assert.NoError(t, os.Mkdir(filepath.Join(cwd, ".glaze"), 0o700))
		chdir(t, cwd)
		t.Setenv("GLAZE_PATH", "")

		_, err := ResolveProfilePath("")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "is a directory, not a profile")
	})

	t.Run("GLAZE_PATH names the profile file, not its directory", func(t *testing.T) {
		dir := t.TempDir()
		profile := filepath.Join(dir, ".glaze")
		assert.NoError(t, os.WriteFile(profile, []byte("session {}"), 0o600))
		chdir(t, t.TempDir())
		t.Setenv("GLAZE_PATH", profile)

		_, err := ResolveProfilePath("")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "GLAZE_PATH must be a directory")
		assert.ErrorContains(t, err, "set GLAZE_PATH to `"+dir+"`")
	})

	t.Run(".glaze in GLAZE_PATH is a directory", func(t *testing.T) {
		dir := t.TempDir()
		assert.NoError(t, os.Mkdir(filepath.Join(dir, ".glaze"), 0o700))
		chdir(t, t.TempDir())
		t.Setenv("GLAZE_PATH", dir)

		_, err := ResolveProfilePath("")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "is a directory, not a profile")
	})
}
