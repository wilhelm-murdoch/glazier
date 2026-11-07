package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rejectedName is a word that is not a profile name, with the part of the message that names the fix.
type rejectedName struct {
	name     string
	fragment string
}

var (
	// acceptedNames are profile names that ValidateProfileName accepts; the table test and the fuzz seeds share them.
	acceptedNames = []string{
		"work",
		"api.v2",
		"_x",
		"2024-q1",
		"Work",
		"help",
		"up",
		"glaze-api",
		"a",
		"0",
		"a-b_c.d",
		strings.Repeat("n", maxProfileNameLength),
	}

	// rejectedNames are words that ValidateProfileName rejects, each with the part of its message that names the fix.
	// The rows follow the order of the checks, so a word with two faults shows which check comes first.
	rejectedNames = []rejectedName{
		{"", "a profile name cannot be empty"},
		{"a/b", "a name cannot contain `/`; for a file, use --profile-path"},
		{"./a", "a name cannot contain `/`"},
		{"../a", "a name cannot contain `/`"},
		{"/abs", "a name cannot contain `/`"},
		{"work/", "a name cannot contain `/`"},
		{"~/x", "a name cannot contain `/`"},
		{"~x", "a name cannot start with `~`; for a file, use --profile-path"},
		{".", "a name cannot start with `.`"},
		{"..", "a name cannot start with `.`"},
		{".hidden", "a name cannot start with `.`"},
		{".glaze", "a name cannot start with `.`"},
		{"-x", "a name cannot start with `-`"},
		{"--detached", "a name cannot start with `-`"},
		{"-1", "a name cannot start with `-`"},
		{"-x.glaze", "a name cannot start with `-`"},
		{"work.glaze", "leave out the `.glaze` suffix (the name is `work`); for a file, use --profile-path work.glaze"},
		{strings.Repeat("n", maxProfileNameLength+1), "a name can have at most 128 characters"},
		{"a b", "a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
		{"a:b", "a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
		{"a;b", "a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
		{"$a", "a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
		{"a*b", "a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
		{"a`b", "a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
		{"a\tb", "a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
		{"a\nb", "a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
		{"wörk", "a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
	}
)

// isolateRegistry points HOME at an empty directory and clears every variable that the registry or a hint reads.
// Without it the registry of the host would leak into a test.
func isolateRegistry(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(RegistryEnv, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("GLAZE_PATH", "")

	return home
}

// tempRegistry isolates the environment and makes an empty profile registry that GLAZE_REGISTRY points at.
func tempRegistry(t *testing.T) string {
	t.Helper()

	isolateRegistry(t)
	dir := filepath.Join(t.TempDir(), "registry")
	require.NoError(t, os.Mkdir(dir, 0o700))
	t.Setenv(RegistryEnv, dir)

	return dir
}

// writeProfile writes `<name>.glaze` in dir and returns its path.
func writeProfile(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name+ProfileSuffix)
	require.NoError(t, os.WriteFile(path, []byte("session {}"), 0o600)) //nolint:gosec // G703: a test file in a temporary directory.

	return path
}

// unreadable takes every permission from dir for the test, or skips the test as root, who can read every directory.
// The permissions come back before the test removes the directory.
func unreadable(t *testing.T, dir string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("root can read every directory")
	}

	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restores a test directory.
}

// inDeletedDirectory changes into a directory and deletes it, so that os.Getwd fails.
// macOS still reports the path of a deleted directory, so the test is skipped there.
func inDeletedDirectory(t *testing.T) {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, os.Mkdir(dir, 0o700))
	chdir(t, dir)
	require.NoError(t, os.Remove(dir))

	if _, err := os.Getwd(); err == nil {
		t.Skip("this system still reports the current directory after it is deleted")
	}
}

func TestRegistryDir(t *testing.T) {
	t.Run("GLAZE_REGISTRY wins over XDG_CONFIG_HOME and HOME", func(t *testing.T) {
		isolateRegistry(t)
		dir := t.TempDir()
		t.Setenv(RegistryEnv, dir)
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		got, err := RegistryDir()
		assert.NoError(t, err)
		assert.Equal(t, dir, got)
	})

	t.Run("GLAZE_REGISTRY expands ~ and cleans the path", func(t *testing.T) {
		home := isolateRegistry(t)

		for registry, want := range map[string]string{
			"~":            home,
			"~/profiles":   filepath.Join(home, "profiles"),
			"~/profiles/":  filepath.Join(home, "profiles"),
			"/srv/./glaze": "/srv/glaze",
		} {
			t.Setenv(RegistryEnv, registry)

			got, err := RegistryDir()
			assert.NoError(t, err, registry)
			assert.Equal(t, want, got, registry)
		}
	})

	t.Run("a relative GLAZE_REGISTRY is an error", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv(RegistryEnv, "profiles")

		_, err := RegistryDir()
		assert.ErrorContains(t, err, "GLAZE_REGISTRY must be an absolute path, but it is `profiles`")
	})

	t.Run("GLAZE_REGISTRY rejects ~user", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv(RegistryEnv, "~root/profiles")

		_, err := RegistryDir()
		assert.ErrorIs(t, err, ErrTildeUser)
		assert.ErrorContains(t, err, "GLAZE_REGISTRY is not valid")
	})

	t.Run("GLAZE_REGISTRY with ~ needs a home directory", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv("HOME", "")
		t.Setenv(RegistryEnv, "~/profiles")

		_, err := RegistryDir()
		assert.ErrorContains(t, err, "GLAZE_REGISTRY is not valid")
		assert.ErrorContains(t, err, "could not expand `~`")
	})

	t.Run("an absolute XDG_CONFIG_HOME gives its glaze directory", func(t *testing.T) {
		isolateRegistry(t)
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)

		got, err := RegistryDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(xdg, "glaze"), got)
	})

	t.Run("an absolute XDG_CONFIG_HOME needs no home directory", func(t *testing.T) {
		isolateRegistry(t)
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		t.Setenv("HOME", "")

		got, err := RegistryDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(xdg, "glaze"), got)
	})

	t.Run("a relative XDG_CONFIG_HOME is ignored", func(t *testing.T) {
		home := isolateRegistry(t)

		for _, xdg := range []string{"config", "./config", "~/config"} {
			t.Setenv("XDG_CONFIG_HOME", xdg)

			got, err := RegistryDir()
			assert.NoError(t, err, xdg)
			assert.Equal(t, filepath.Join(home, ".config", "glaze"), got, xdg)
		}
	})

	t.Run("nothing set gives .config/glaze in the home directory", func(t *testing.T) {
		home := isolateRegistry(t)
		require.NoError(t, os.Unsetenv(RegistryEnv))
		require.NoError(t, os.Unsetenv("XDG_CONFIG_HOME"))

		got, err := RegistryDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".config", "glaze"), got)
	})

	t.Run("an empty GLAZE_REGISTRY is the same as an unset one", func(t *testing.T) {
		isolateRegistry(t)
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)

		got, err := RegistryDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(xdg, "glaze"), got)
	})

	t.Run("an empty XDG_CONFIG_HOME is the same as an unset one", func(t *testing.T) {
		home := isolateRegistry(t)
		require.NoError(t, os.Unsetenv(RegistryEnv))

		got, err := RegistryDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".config", "glaze"), got)
	})

	t.Run("fails without a home directory and names the variables to set", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv("HOME", "")

		_, err := RegistryDir()
		assert.ErrorIs(t, err, ErrRegistryUnusable)

		for _, variable := range []string{"HOME", "XDG_CONFIG_HOME", RegistryEnv} {
			assert.ErrorContains(t, err, variable)
		}
	})
}

func TestIgnoredConfigHome(t *testing.T) {
	t.Run("returns a relative XDG_CONFIG_HOME", func(t *testing.T) {
		isolateRegistry(t)

		for _, xdg := range []string{"config", "./config", "~/config"} {
			t.Setenv("XDG_CONFIG_HOME", xdg)

			value, ignored := IgnoredConfigHome()
			assert.True(t, ignored, xdg)
			assert.Equal(t, xdg, value)
		}
	})

	t.Run("is false for an absolute, an empty and an unset XDG_CONFIG_HOME", func(t *testing.T) {
		isolateRegistry(t)

		for _, xdg := range []string{t.TempDir(), ""} {
			t.Setenv("XDG_CONFIG_HOME", xdg)

			_, ignored := IgnoredConfigHome()
			assert.False(t, ignored, xdg)
		}

		require.NoError(t, os.Unsetenv("XDG_CONFIG_HOME"))

		_, ignored := IgnoredConfigHome()
		assert.False(t, ignored)
	})

	t.Run("is false when GLAZE_REGISTRY replaces the XDG rule", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv("XDG_CONFIG_HOME", "config")
		t.Setenv(RegistryEnv, t.TempDir())

		value, ignored := IgnoredConfigHome()
		assert.False(t, ignored)
		assert.Empty(t, value)
	})
}

func TestValidateProfileName(t *testing.T) {
	for _, name := range acceptedNames {
		assert.NoError(t, ValidateProfileName(name), name)
	}

	for _, row := range rejectedNames {
		err := ValidateProfileName(row.name)
		assert.ErrorIs(t, err, ErrInvalidProfileName, row.name)
		assert.ErrorContains(t, err, row.fragment, row.name)

		if row.name != "" {
			assert.ErrorContains(t, err, "`"+row.name+"` is not a profile name", row.name)
		}
	}
}

func TestRegistryProfilePath(t *testing.T) {
	t.Run("joins the name to the registry without reading the file system", func(t *testing.T) {
		isolateRegistry(t)
		dir := filepath.Join(t.TempDir(), "missing")
		t.Setenv(RegistryEnv, dir)

		got, err := RegistryProfilePath("work")
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "work.glaze"), got)
	})

	t.Run("uses the default registry under the home directory", func(t *testing.T) {
		home := isolateRegistry(t)

		got, err := RegistryProfilePath("api.v2")
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".config", "glaze", "api.v2.glaze"), got)
	})

	t.Run("rejects a bad name before it looks at the registry", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv(RegistryEnv, "profiles")

		for _, name := range []string{"a/b", "work.glaze", "", "-x"} {
			_, err := RegistryProfilePath(name)
			assert.ErrorIs(t, err, ErrInvalidProfileName, name)
			assert.NotErrorIs(t, err, ErrProfileNotFound, name)
		}
	})

	t.Run("reports a registry that cannot be found", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv(RegistryEnv, "profiles")

		_, err := RegistryProfilePath("work")
		assert.ErrorContains(t, err, "GLAZE_REGISTRY must be an absolute path")
	})
}

func TestResolveProfilePathByName(t *testing.T) {
	t.Run("finds the profile in the registry", func(t *testing.T) {
		dir := tempRegistry(t)
		profile := writeProfile(t, dir, "work")

		resolved, err := ResolveProfilePath("", "work")
		assert.NoError(t, err)
		assert.Equal(t, profile, resolved)
	})

	t.Run("finds the profile in the default registry under the home directory", func(t *testing.T) {
		home := isolateRegistry(t)
		dir := filepath.Join(home, ".config", "glaze")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		profile := writeProfile(t, dir, "api.v2")

		resolved, err := ResolveProfilePath("", "api.v2")
		assert.NoError(t, err)
		assert.Equal(t, profile, resolved)
	})

	t.Run("the name wins over a profile path", func(t *testing.T) {
		dir := tempRegistry(t)
		profile := writeProfile(t, dir, "work")

		resolved, err := ResolveProfilePath("/definitely/not/here.glaze", "work")
		assert.NoError(t, err)
		assert.Equal(t, profile, resolved)
	})

	t.Run("an empty name is the search without a name", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv(RegistryEnv, "profiles")
		cwd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(cwd, ".glaze"), []byte("session {}"), 0o600))
		chdir(t, cwd)

		// macOS reports the current directory through a resolved symlink, so the expected path comes from Getwd.
		cwd, err := os.Getwd()
		require.NoError(t, err)

		resolved, err := ResolveProfilePath("", "")
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(cwd, ".glaze"), resolved)
	})

	t.Run("returns the link path for a symlinked profile", func(t *testing.T) {
		dir := tempRegistry(t)
		target := writeProfile(t, t.TempDir(), "real")
		link := filepath.Join(dir, "work.glaze")
		require.NoError(t, os.Symlink(target, link))

		resolved, err := ResolveProfilePath("", "work")
		assert.NoError(t, err)
		assert.Equal(t, link, resolved)
	})

	t.Run("follows a symlinked registry", func(t *testing.T) {
		dir := tempRegistry(t)
		writeProfile(t, dir, "work")
		link := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(dir, link))
		t.Setenv(RegistryEnv, link)

		resolved, err := ResolveProfilePath("", "work")
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(link, "work.glaze"), resolved)
	})

	t.Run("never reads the current directory", func(t *testing.T) {
		dir := tempRegistry(t)
		cwd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(cwd, ".glaze"), []byte("session {}"), 0o600))
		chdir(t, cwd)

		resolved, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.Equal(t, filepath.Join(dir, "work.glaze"), resolved)
	})

	t.Run("a missing profile names the file, the registry and the fixes", func(t *testing.T) {
		dir := tempRegistry(t)
		chdir(t, t.TempDir())

		resolved, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.Equal(t, filepath.Join(dir, "work.glaze"), resolved)
		assert.ErrorContains(t, err, "no profile named `work` in the profile registry; `"+filepath.Join(dir, "work.glaze")+"` does not exist")
		assert.ErrorContains(t, err, "\n - `glaze ls --profiles` lists the profiles in `"+dir+"`")
		assert.ErrorContains(t, err, "\n - --profile-path reads a profile from any path")
		assert.NotContains(t, err.Error(), "current directory")
		assert.NotContains(t, err.Error(), "GLAZE_PATH")
	})

	t.Run("a missing profile points at the file in the current directory", func(t *testing.T) {
		tempRegistry(t)
		cwd := t.TempDir()
		writeProfile(t, cwd, "work")
		chdir(t, cwd)

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "\n - `work.glaze` is in the current directory; a name never reads the current directory, so use --profile-path work.glaze")
		assert.NotContains(t, err.Error(), "GLAZE_PATH")
	})

	t.Run("a missing profile points at the file in GLAZE_PATH", func(t *testing.T) {
		dir := tempRegistry(t)
		glazePath := t.TempDir()
		profile := writeProfile(t, glazePath, "work")
		chdir(t, t.TempDir())
		t.Setenv("GLAZE_PATH", glazePath)

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "\n - `"+profile+"` exists in GLAZE_PATH; GLAZE_PATH is not the profile registry, so move the file to `"+dir+"` or set GLAZE_REGISTRY")
		assert.NotContains(t, err.Error(), "current directory")
	})

	t.Run("expands ~ in GLAZE_PATH for the hint", func(t *testing.T) {
		home := isolateRegistry(t)
		t.Setenv(RegistryEnv, t.TempDir())
		require.NoError(t, os.Mkdir(filepath.Join(home, "gp"), 0o700))
		profile := writeProfile(t, filepath.Join(home, "gp"), "work")
		chdir(t, t.TempDir())
		t.Setenv("GLAZE_PATH", "~/gp")

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "`"+profile+"` exists in GLAZE_PATH")
	})

	t.Run("skips the current directory hint without a current directory", func(t *testing.T) {
		dir := tempRegistry(t)
		glazePath := t.TempDir()
		profile := writeProfile(t, glazePath, "work")
		t.Setenv("GLAZE_PATH", glazePath)
		inDeletedDirectory(t)

		resolved, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.Equal(t, filepath.Join(dir, "work.glaze"), resolved)
		assert.NotContains(t, err.Error(), "current directory")
		assert.ErrorContains(t, err, "`"+profile+"` exists in GLAZE_PATH")
	})

	t.Run("finds the profile without a current directory", func(t *testing.T) {
		dir := tempRegistry(t)
		profile := writeProfile(t, dir, "work")
		inDeletedDirectory(t)

		resolved, err := ResolveProfilePath("", "work")
		assert.NoError(t, err)
		assert.Equal(t, profile, resolved)
	})

	t.Run("says when the registry does not exist", func(t *testing.T) {
		isolateRegistry(t)
		dir := filepath.Join(t.TempDir(), "registry")
		t.Setenv(RegistryEnv, dir)
		chdir(t, t.TempDir())

		resolved, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.Equal(t, filepath.Join(dir, "work.glaze"), resolved)
		assert.ErrorContains(t, err, "no profile named `work`; the profile registry `"+dir+"` does not exist")
		assert.ErrorContains(t, err, "\n - or create it with `mkdir -p "+dir+"` and put `work.glaze` in it")
		assert.ErrorContains(t, err, "\n - --profile-path reads a profile from any path")
		assert.NotContains(t, err.Error(), "current directory")
	})

	t.Run("a missing registry still points at the file in the current directory", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv(RegistryEnv, filepath.Join(t.TempDir(), "registry"))
		cwd := t.TempDir()
		writeProfile(t, cwd, "work")
		chdir(t, cwd)

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "the profile registry `")
		assert.ErrorContains(t, err, "\n - `work.glaze` is in the current directory; a name never reads the current directory, so use --profile-path work.glaze")
	})

	t.Run("says when the registry is a file and names the variable that chose it", func(t *testing.T) {
		t.Run("GLAZE_REGISTRY", func(t *testing.T) {
			isolateRegistry(t)
			file := filepath.Join(t.TempDir(), "registry")
			require.NoError(t, os.WriteFile(file, []byte("session {}"), 0o600))
			t.Setenv(RegistryEnv, file)

			_, err := ResolveProfilePath("", "work")
			assert.ErrorIs(t, err, ErrProfileNotFound)
			assert.ErrorContains(t, err, "`"+file+"` is a file, not the profile registry directory; set GLAZE_REGISTRY to a directory")
		})

		t.Run("XDG_CONFIG_HOME", func(t *testing.T) {
			isolateRegistry(t)
			xdg := t.TempDir()
			file := filepath.Join(xdg, "glaze")
			require.NoError(t, os.WriteFile(file, []byte("session {}"), 0o600))
			t.Setenv("XDG_CONFIG_HOME", xdg)

			_, err := ResolveProfilePath("", "work")
			assert.ErrorIs(t, err, ErrProfileNotFound)
			assert.ErrorContains(t, err, "`"+file+"` is a file, not the profile registry directory; set XDG_CONFIG_HOME to a directory, or set GLAZE_REGISTRY")
		})

		t.Run("HOME", func(t *testing.T) {
			home := isolateRegistry(t)
			require.NoError(t, os.Mkdir(filepath.Join(home, ".config"), 0o700))
			file := filepath.Join(home, ".config", "glaze")
			require.NoError(t, os.WriteFile(file, []byte("session {}"), 0o600))

			_, err := ResolveProfilePath("", "work")
			assert.ErrorIs(t, err, ErrProfileNotFound)
			assert.ErrorContains(t, err, "`"+file+"` is a file, not the profile registry directory; move it away, or set XDG_CONFIG_HOME or GLAZE_REGISTRY to a directory")
		})
	})

	t.Run("says when a part of the registry path is a file", func(t *testing.T) {
		t.Run("GLAZE_REGISTRY", func(t *testing.T) {
			isolateRegistry(t)
			file := filepath.Join(t.TempDir(), "config")
			require.NoError(t, os.WriteFile(file, []byte("session {}"), 0o600))
			dir := filepath.Join(file, "registry")
			t.Setenv(RegistryEnv, dir)

			resolved, err := ResolveProfilePath("", "work")
			assert.ErrorIs(t, err, ErrProfileNotFound)
			assert.Equal(t, filepath.Join(dir, "work.glaze"), resolved)
			assert.ErrorContains(t, err, "`"+dir+"` cannot be the profile registry, because a part of its path is a file; set GLAZE_REGISTRY to a directory")
		})

		t.Run("HOME", func(t *testing.T) {
			home := isolateRegistry(t)
			require.NoError(t, os.WriteFile(filepath.Join(home, ".config"), []byte("session {}"), 0o600))
			dir := filepath.Join(home, ".config", "glaze")

			_, err := ResolveProfilePath("", "work")
			assert.ErrorIs(t, err, ErrProfileNotFound)
			assert.ErrorContains(t, err, "`"+dir+"` cannot be the profile registry, because a part of its path is a file; move it away, or set XDG_CONFIG_HOME or GLAZE_REGISTRY to a directory")
		})
	})

	t.Run("says when the registry is a dangling symlink", func(t *testing.T) {
		isolateRegistry(t)
		target := filepath.Join(t.TempDir(), "gone")
		link := filepath.Join(t.TempDir(), "registry")
		require.NoError(t, os.Symlink(target, link))
		t.Setenv(RegistryEnv, link)

		resolved, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorIs(t, err, ErrRegistryUnusable)
		assert.Equal(t, filepath.Join(link, "work.glaze"), resolved)
		assert.ErrorContains(t, err, "`"+link+"` is a symlink to `"+target+"`, which does not exist; set GLAZE_REGISTRY to a directory")
		assert.NotContains(t, err.Error(), "mkdir -p")
	})

	t.Run("says when the registry cannot be read", func(t *testing.T) {
		isolateRegistry(t)
		parent := t.TempDir()
		dir := filepath.Join(parent, "registry")
		require.NoError(t, os.Mkdir(dir, 0o700))
		t.Setenv(RegistryEnv, dir)
		unreadable(t, parent)

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorIs(t, err, os.ErrPermission)
		assert.ErrorIs(t, err, ErrRegistryUnusable)
		assert.ErrorContains(t, err, "could not read `"+dir+"`")
		assert.ErrorContains(t, err, "permission denied")
	})

	t.Run("says when the profile cannot be read", func(t *testing.T) {
		dir := tempRegistry(t)
		profile := writeProfile(t, dir, "work")
		unreadable(t, dir)

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorIs(t, err, os.ErrPermission)
		assert.ErrorContains(t, err, "could not read `"+profile+"`")
		assert.ErrorContains(t, err, "permission denied")
	})

	t.Run("says when the entry is a directory", func(t *testing.T) {
		dir := tempRegistry(t)
		entry := filepath.Join(dir, "work.glaze")
		require.NoError(t, os.Mkdir(entry, 0o700))

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "`"+entry+"` is a directory, not a profile")
	})

	t.Run("says when the entry is a symlink to a directory", func(t *testing.T) {
		dir := tempRegistry(t)
		link := filepath.Join(dir, "work.glaze")
		require.NoError(t, os.Symlink(t.TempDir(), link))

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "`"+link+"` is a directory, not a profile")
	})

	t.Run("names the target of a dangling symlink", func(t *testing.T) {
		dir := tempRegistry(t)
		link := filepath.Join(dir, "work.glaze")
		target := filepath.Join(dir, "gone.glaze")
		require.NoError(t, os.Symlink(target, link))

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "`"+link+"` is a symlink to `"+target+"`, which does not exist")
	})

	t.Run("rejects a bad name without reading the registry", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv(RegistryEnv, "profiles")

		for _, name := range []string{"a/b", "work.glaze", "-x", "a b"} {
			_, err := ResolveProfilePath("", name)
			assert.ErrorIs(t, err, ErrInvalidProfileName, name)
			assert.NotErrorIs(t, err, ErrProfileNotFound, name)
		}
	})

	t.Run("wraps a registry that cannot be found", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv("HOME", "")

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "set HOME, XDG_CONFIG_HOME or GLAZE_REGISTRY")
	})

	t.Run("wraps a relative GLAZE_REGISTRY", func(t *testing.T) {
		isolateRegistry(t)
		t.Setenv(RegistryEnv, "profiles")

		_, err := ResolveProfilePath("", "work")
		assert.ErrorIs(t, err, ErrProfileNotFound)
		assert.ErrorContains(t, err, "GLAZE_REGISTRY must be an absolute path")
	})
}

func TestListProfiles(t *testing.T) {
	t.Run("lists the profiles sorted by name and skips what is not one", func(t *testing.T) {
		dir := t.TempDir()
		work := writeProfile(t, dir, "work")
		api := writeProfile(t, dir, "api")
		apiV2 := writeProfile(t, dir, "api-v2")
		link := filepath.Join(dir, "mid.glaze")
		require.NoError(t, os.Symlink(writeProfile(t, t.TempDir(), "real"), link))
		writeProfile(t, dir, ".hidden")
		writeProfile(t, dir, "")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("notes"), 0o600))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "sub.glaze"), 0o700))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))

		profiles, skipped, err := ListProfiles(dir)
		assert.NoError(t, err)
		assert.Equal(t, []Profile{
			{Name: "api", Path: api},
			{Name: "api-v2", Path: apiV2},
			{Name: "mid", Path: link},
			{Name: "work", Path: work},
		}, profiles)
		assert.Empty(t, skipped)
	})

	t.Run("returns the entries that up cannot use with a reason", func(t *testing.T) {
		dir := t.TempDir()
		bad := writeProfile(t, dir, "bad name")
		dash := writeProfile(t, dir, "-x")
		dangling := filepath.Join(dir, "gone.glaze")
		require.NoError(t, os.Symlink(filepath.Join(dir, "missing.glaze"), dangling))
		fifo := filepath.Join(dir, "pipe.glaze")
		require.NoError(t, syscall.Mkfifo(fifo, 0o600))

		// A symlink to a directory is left out like a directory, so it gets no reason.
		require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(dir, "todir.glaze")))

		profiles, skipped, err := ListProfiles(dir)
		assert.NoError(t, err)
		assert.Empty(t, profiles)
		assert.Equal(t, []SkippedEntry{
			{File: dash, Reason: "`-x` is not a profile name: a name cannot start with `-`"},
			{File: bad, Reason: "`bad name` is not a profile name: a name can contain only ASCII letters, digits, `.`, `_` and `-`"},
			{File: dangling, Reason: "a symlink to `" + filepath.Join(dir, "missing.glaze") + "`, which does not exist"},
			{File: fifo, Reason: "not a regular file"},
		}, skipped)
	})

	t.Run("returns nothing for an empty registry", func(t *testing.T) {
		profiles, skipped, err := ListProfiles(t.TempDir())
		assert.NoError(t, err)
		assert.Empty(t, profiles)
		assert.Empty(t, skipped)
	})

	t.Run("reports a missing registry as not existing", func(t *testing.T) {
		_, _, err := ListProfiles(filepath.Join(t.TempDir(), "missing"))
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("reports a registry that is a dangling symlink as unusable, not as missing", func(t *testing.T) {
		isolateRegistry(t)
		target := filepath.Join(t.TempDir(), "gone")
		link := filepath.Join(t.TempDir(), "registry")
		require.NoError(t, os.Symlink(target, link))
		t.Setenv(RegistryEnv, link)

		// `ls --profiles` prints nothing for a missing registry, so a dangling symlink must not look missing.
		_, _, err := ListProfiles(link)
		assert.ErrorIs(t, err, ErrRegistryUnusable)
		assert.NotErrorIs(t, err, fs.ErrNotExist)
		assert.ErrorContains(t, err, "`"+link+"` is a symlink to `"+target+"`, which does not exist; set GLAZE_REGISTRY to a directory")
	})

	t.Run("reports a registry that is a file", func(t *testing.T) {
		isolateRegistry(t)
		file := filepath.Join(t.TempDir(), "registry")
		require.NoError(t, os.WriteFile(file, []byte("session {}"), 0o600))

		_, _, err := ListProfiles(file)
		assert.ErrorContains(t, err, "`"+file+"` is a file, not the profile registry directory; move it away, or set XDG_CONFIG_HOME or GLAZE_REGISTRY to a directory")
	})

	t.Run("reports a registry with a file in its path", func(t *testing.T) {
		isolateRegistry(t)
		file := filepath.Join(t.TempDir(), "config")
		require.NoError(t, os.WriteFile(file, []byte("session {}"), 0o600))
		dir := filepath.Join(file, "glaze")
		t.Setenv(RegistryEnv, dir)

		_, _, err := ListProfiles(dir)
		assert.ErrorContains(t, err, "`"+dir+"` cannot be the profile registry, because a part of its path is a file; set GLAZE_REGISTRY to a directory")
	})

	t.Run("reports a registry that cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		writeProfile(t, dir, "work")
		unreadable(t, dir)

		_, _, err := ListProfiles(dir)
		assert.ErrorIs(t, err, os.ErrPermission)
	})
}

// FuzzValidateProfileName sends arbitrary words through the name rule.
// An accepted name must stay one file inside the registry, and a rejected word must say so with ErrInvalidProfileName.
func FuzzValidateProfileName(f *testing.F) {
	for _, name := range acceptedNames {
		f.Add(name)
	}

	for _, row := range rejectedNames {
		f.Add(row.name)
	}

	f.Fuzz(func(t *testing.T, name string) {
		err := ValidateProfileName(name)
		if err != nil {
			if !errors.Is(err, ErrInvalidProfileName) {
				t.Fatalf("rejected %q without ErrInvalidProfileName: %v", name, err)
			}

			return
		}

		file := name + ProfileSuffix
		path := filepath.Join("/registry", file)
		switch {
		case strings.ContainsRune(name, filepath.Separator):
			t.Fatalf("accepted %q, which contains a path separator", name)
		case strings.HasPrefix(name, "-") || strings.HasPrefix(name, "."):
			t.Fatalf("accepted %q, which starts with `-` or `.`", name)
		case strings.HasSuffix(name, ProfileSuffix):
			t.Fatalf("accepted %q, which ends with the suffix", name)
		case len(name) > maxProfileNameLength:
			t.Fatalf("accepted %q, which is longer than %d characters", name, maxProfileNameLength)
		case filepath.Dir(path) != "/registry" || filepath.Base(path) != file:
			t.Fatalf("accepted %q, which leaves the registry: %s", name, path)
		}
	})
}
