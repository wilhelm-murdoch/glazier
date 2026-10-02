package parser

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inDeletedDirectory changes into a directory and deletes it, so that os.Getwd fails.
// macOS still reports the path of a deleted directory, so the test is skipped there.
func inDeletedDirectory(t *testing.T) {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, os.Mkdir(dir, 0o700))
	t.Chdir(dir)
	require.NoError(t, os.Remove(dir))

	if _, err := os.Getwd(); err == nil {
		t.Skip("this system still reports the current directory after it is deleted")
	}
}

func TestPathUnavailable(t *testing.T) {
	cause := errors.New("getwd: no such file or directory")

	t.Run("names the first use of path", func(t *testing.T) {
		p, diags := New(writeGlaze(t, profileWithName("", "name = path.base")))
		require.False(t, diags.HasErrors())

		diags = p.pathUnavailable(cause)
		require.True(t, diags.HasErrors())
		assert.Equal(t, "Current directory not available", diags[0].Summary)
		assert.Contains(t, diags[0].Detail, "getwd: no such file or directory")
		assert.Equal(t, 3, diags[0].Subject.Start.Line)
	})

	t.Run("finds path inside a template", func(t *testing.T) {
		p, diags := New(writeGlaze(t, profileWithName("", `name = "gig-${path.base}"`)))
		require.False(t, diags.HasErrors())
		assert.True(t, p.pathUnavailable(cause).HasErrors())
	})

	t.Run("is no error for a profile that does not use path", func(t *testing.T) {
		p, diags := New(writeGlaze(t, profileWithName("", `name = "demo"`)))
		require.False(t, diags.HasErrors())
		assert.Empty(t, p.pathUnavailable(cause))
	})
}

func TestVariableContextWithoutACurrentDirectory(t *testing.T) {
	plain := writeGlaze(t, profileWithName("", `name = "demo"`))
	usesPath := writeGlaze(t, profileWithName("", "name = path.base"))

	inDeletedDirectory(t)

	p, diags := New(plain)
	require.False(t, diags.HasErrors())
	_, diags = p.VariableContext(nil, "", true)
	assert.False(t, diags.HasErrors(), "a profile that does not use path needs no current directory: %s", diags.Error())

	p, diags = New(usesPath)
	require.False(t, diags.HasErrors())
	_, diags = p.VariableContext(nil, "", true)
	require.True(t, diags.HasErrors())
	assert.Equal(t, "Current directory not available", diags[0].Summary)
}
