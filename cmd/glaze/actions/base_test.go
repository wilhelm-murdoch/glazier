package actions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
)

// newRegistry points HOME at a new temporary directory and clears the variables that move the profile registry or add a profile source.
// It returns the profile registry, which does not exist yet, so a test can also cover a missing one.
func newRegistry(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GLAZE_REGISTRY", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("GLAZE_PATH", "")

	return filepath.Join(home, ".config", "glaze")
}

// writeRegistryProfile writes a profile with these contents into the profile registry, which it creates when needed, and returns its path.
func writeRegistryProfile(t *testing.T, registry, name, contents string) string {
	t.Helper()

	assert.NoError(t, os.MkdirAll(registry, 0o700))
	path := filepath.Join(registry, name+files.ProfileSuffix)
	assert.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	return path
}

// flagArgs returns each flag as `--key=value`, so a bool flag does not read the next word as a profile name.
func flagArgs(flags map[string]string) []string {
	args := make([]string, 0, len(flags))
	for k, v := range flags {
		args = append(args, "--"+k+"="+v)
	}

	return args
}

// captureDiagnostics sends the diagnostics of the action to a buffer, with the source of the profile for the snippet.
func captureDiagnostics(base *ActionBase) *bytes.Buffer {
	var out bytes.Buffer
	base.DiagnosticsManager.Writer = hcl.NewDiagnosticTextWriter(&out, map[string]*hcl.File{base.ProfilePath: base.Parser.File}, 0, false)

	return &out
}

// runBase runs NewActionBase through a real cli.Command with these arguments, so a flag and a profile name reach it as they do from main.
func runBase(t *testing.T, args ...string) (*ActionBase, error) {
	t.Helper()

	return runBaseAt(t, "critical", args...)
}

// runBaseAt is runBase with the log level of the action, for a test that reads its log lines.
func runBaseAt(t *testing.T, logLevel string, args ...string) (*ActionBase, error) {
	t.Helper()

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
			base, baseErr = NewActionBase(c, logLevel)
			return nil
		},
	}

	assert.NoError(t, cmd.Run(context.Background(), append([]string{"test"}, args...)))

	return base, baseErr
}

// runWithProfile runs NewActionBase on a profile with these contents, through --profile-path.
func runWithProfile(t *testing.T, contents string) (*ActionBase, error) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, ".glaze")
	assert.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	return runBase(t, "--profile-path", path)
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

	t.Run("keeps an empty profile name for a path", func(t *testing.T) {
		base, err := runWithProfile(t, validProfile)

		assert.NoError(t, err)
		if assert.NotNil(t, base) {
			assert.Empty(t, base.ProfileName)
		}
	})
}

func TestNewActionBaseWithAProfileName(t *testing.T) {
	t.Run("reads the profile of the name from the profile registry", func(t *testing.T) {
		registry := newRegistry(t)
		path := writeRegistryProfile(t, registry, "work", validProfile)

		base, err := runBase(t, "work")

		assert.NoError(t, err)
		if assert.NotNil(t, base) {
			assert.Equal(t, path, base.ProfilePath)
			assert.Equal(t, "work", base.ProfileName)
			assert.Equal(t, registry, base.profileDir())
		}
	})

	t.Run("returns the not-found sentinel for a name without a profile", func(t *testing.T) {
		registry := newRegistry(t)
		writeRegistryProfile(t, registry, "other", validProfile)

		base, err := runBase(t, "work")

		assert.Nil(t, base)
		assert.ErrorIs(t, err, files.ErrProfileNotFound)
		assert.ErrorContains(t, err, "no profile named `work` in the profile registry")
		assert.ErrorContains(t, err, "`"+filepath.Join(registry, "work.glaze")+"` does not exist")
		assert.ErrorContains(t, err, "`glaze ls --profiles` lists the profiles in `"+registry+"`")
		assert.ErrorContains(t, err, "--profile-path reads a profile from any path")
	})

	t.Run("says how to create a missing profile registry", func(t *testing.T) {
		registry := newRegistry(t)

		base, err := runBase(t, "work")

		assert.Nil(t, base)
		assert.ErrorIs(t, err, files.ErrProfileNotFound)
		assert.ErrorContains(t, err, "no profile named `work`; the profile registry `"+registry+"` does not exist")
		assert.ErrorContains(t, err, "create it with `mkdir -p "+registry+"` and put `work.glaze` in it")
		assert.ErrorContains(t, err, "--profile-path reads a profile from any path")
	})

	t.Run("warns once when a relative XDG_CONFIG_HOME is ignored", func(t *testing.T) {
		registry := newRegistry(t)
		writeRegistryProfile(t, registry, "work", validProfile)
		t.Setenv("XDG_CONFIG_HOME", "rel")

		var (
			base *ActionBase
			err  error
		)

		stderr := capture(t, &os.Stderr, func() { base, err = runBaseAt(t, "warn", "work") })

		assert.NoError(t, err)
		if assert.NotNil(t, base) {
			assert.Equal(t, filepath.Join(registry, "work.glaze"), base.ProfilePath)
		}

		assert.Equal(t, 1, strings.Count(stderr, "XDG_CONFIG_HOME is not absolute, so the profile registry is ~/.config/glaze"))
		assert.Contains(t, stderr, "XDG_CONFIG_HOME=rel")
	})

	t.Run("warns about a relative XDG_CONFIG_HOME before it reports a missing profile", func(t *testing.T) {
		registry := newRegistry(t)
		writeRegistryProfile(t, registry, "other", validProfile)
		t.Setenv("XDG_CONFIG_HOME", "rel")

		var (
			base *ActionBase
			err  error
		)

		stderr := capture(t, &os.Stderr, func() { base, err = runBaseAt(t, "warn", "work") })

		// The warning comes before the error, because the profile can be in `rel/glaze`, which the registry rule ignores.
		assert.Nil(t, base)
		assert.ErrorIs(t, err, files.ErrProfileNotFound)
		assert.ErrorContains(t, err, "no profile named `work` in the profile registry")
		assert.Equal(t, 1, strings.Count(stderr, "WRN XDG_CONFIG_HOME is not absolute, so the profile registry is ~/.config/glaze"))
		assert.Contains(t, stderr, "XDG_CONFIG_HOME=rel")
	})

	t.Run("does not warn about a relative XDG_CONFIG_HOME for a path", func(t *testing.T) {
		newRegistry(t)
		t.Setenv("XDG_CONFIG_HOME", "rel")
		path := filepath.Join(t.TempDir(), ".glaze")
		assert.NoError(t, os.WriteFile(path, []byte(validProfile), 0o600))

		var err error
		stderr := capture(t, &os.Stderr, func() { _, err = runBaseAt(t, "warn", "--profile-path", path) })

		assert.NoError(t, err)
		assert.Empty(t, stderr)
	})

	t.Run("reads only the profile registry and points at a file of that name elsewhere", func(t *testing.T) {
		registry := newRegistry(t)
		writeRegistryProfile(t, registry, "other", validProfile)

		// A `work.glaze` in the current directory and in GLAZE_PATH is no match for a name; both appear only as a hint.
		cwd := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(cwd, "work.glaze"), []byte(validProfile), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(cwd, ".glaze"), []byte(validProfile), 0o600))
		t.Chdir(cwd)

		glazePath := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(glazePath, "work.glaze"), []byte(validProfile), 0o600))
		t.Setenv("GLAZE_PATH", glazePath)

		base, err := runBase(t, "work")

		assert.Nil(t, base)
		assert.ErrorIs(t, err, files.ErrProfileNotFound)
		assert.ErrorContains(t, err, "`work.glaze` is in the current directory; a name never reads the current directory, so use --profile-path work.glaze")
		assert.ErrorContains(t, err, "`"+filepath.Join(glazePath, "work.glaze")+"` exists in GLAZE_PATH; GLAZE_PATH is not the profile registry")
	})

	t.Run("rejects a word that is not a profile name", func(t *testing.T) {
		newRegistry(t)

		base, err := runBase(t, "a/b")

		assert.Nil(t, base)
		assert.ErrorIs(t, err, files.ErrInvalidProfileName)
		assert.NotErrorIs(t, err, files.ErrProfileNotFound)
		assert.ErrorContains(t, err, "a name cannot contain `/`; for a file, use --profile-path")
	})

	t.Run("names the registry path in a syntax error", func(t *testing.T) {
		registry := newRegistry(t)
		path := writeRegistryProfile(t, registry, "work", "session {\n  window {\n")

		var (
			base *ActionBase
			err  error
		)

		stderr := capture(t, &os.Stderr, func() { base, err = runBase(t, "work") })

		assert.Nil(t, base)
		assert.ErrorIs(t, err, diagnostics.ErrHasDiagnostics)
		assert.Contains(t, stderr, path)
	})
}
