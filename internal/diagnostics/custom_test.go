package diagnostics

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func errorDiag(summary string) *hcl.Diagnostic {
	return &hcl.Diagnostic{Severity: hcl.DiagError, Summary: summary}
}

func warnDiag(summary string) *hcl.Diagnostic {
	return &hcl.Diagnostic{Severity: hcl.DiagWarning, Summary: summary}
}

func TestDiagnosticsManagerExtendAndAppend(t *testing.T) {
	dm := New("profile.glaze", nil)

	dm.Append(errorDiag("first"))
	dm.Extend(hcl.Diagnostics{errorDiag("second"), warnDiag("third")})

	assert.Len(t, dm.Diagnostics, 3)
	assert.True(t, dm.HasErrors())
}

func TestDiagnosticsManagerWrite(t *testing.T) {
	t.Run("returns the sentinel error when error diagnostics are present", func(t *testing.T) {
		dm := New("profile.glaze", nil)
		dm.Append(errorDiag("boom"))

		err := dm.Write()
		assert.ErrorIs(t, err, ErrHasDiagnostics)
	})

	t.Run("returns nil when only warnings are present", func(t *testing.T) {
		dm := New("profile.glaze", nil)
		dm.Append(warnDiag("careful"))

		assert.NoError(t, dm.Write())
	})

	t.Run("returns nil when there are no diagnostics", func(t *testing.T) {
		dm := New("profile.glaze", nil)
		assert.NoError(t, dm.Write())
	})
}

func TestContainsDiagnostic(t *testing.T) {
	list := []string{"tiled", "even-horizontal"}

	t.Run("no diagnostic for a value within the list", func(t *testing.T) {
		assert.Empty(t, ContainsDiagnostic("layout", cty.StringVal("tiled"), list))
	})

	t.Run("no diagnostic for a null value", func(t *testing.T) {
		assert.Empty(t, ContainsDiagnostic("layout", cty.NullVal(cty.String), list))
	})

	t.Run("diagnostic for a value outside the list", func(t *testing.T) {
		diags := ContainsDiagnostic("layout", cty.StringVal("nope"), list)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags[0].Detail, "not supported")
	})
}

func TestLayoutDiagnostic(t *testing.T) {
	list := []string{"tiled", "even-horizontal"}

	t.Run("no diagnostic for a named preset", func(t *testing.T) {
		assert.Empty(t, LayoutDiagnostic("layout", cty.StringVal("tiled"), list))
	})

	t.Run("no diagnostic for a raw coordinate string", func(t *testing.T) {
		assert.Empty(t, LayoutDiagnostic("layout", cty.StringVal("bb62,80x24,0,0"), list))
	})

	t.Run("no diagnostic for a null value", func(t *testing.T) {
		assert.Empty(t, LayoutDiagnostic("layout", cty.NullVal(cty.String), list))
	})

	t.Run("diagnostic for a value that is neither preset nor layout string", func(t *testing.T) {
		diags := LayoutDiagnostic("layout", cty.StringVal("not-a-layout"), list)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags[0].Detail, "not a supported preset")
	})
}

func TestDirectoryDiagnostic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	assert.NoError(t, os.WriteFile(file, []byte("x"), 0o644))

	t.Run("no diagnostic for an existing directory", func(t *testing.T) {
		assert.Empty(t, DirectoryDiagnostic("starting_directory", cty.StringVal(dir)))
	})

	t.Run("no diagnostic for a null value", func(t *testing.T) {
		assert.Empty(t, DirectoryDiagnostic("starting_directory", cty.NullVal(cty.String)))
	})

	t.Run("diagnostic when the path is a file", func(t *testing.T) {
		diags := DirectoryDiagnostic("starting_directory", cty.StringVal(file))
		assert.True(t, diags.HasErrors())
	})

	t.Run("diagnostic when the path does not exist", func(t *testing.T) {
		diags := DirectoryDiagnostic("starting_directory", cty.StringVal(filepath.Join(dir, "nope")))
		assert.True(t, diags.HasErrors())
	})
}

func TestFileDiagnostic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	assert.NoError(t, os.WriteFile(file, []byte("x"), 0o644))

	t.Run("no diagnostic for an existing file", func(t *testing.T) {
		assert.Empty(t, FileDiagnostic("path", cty.StringVal(file)))
	})

	t.Run("no diagnostic for a null value", func(t *testing.T) {
		assert.Empty(t, FileDiagnostic("path", cty.NullVal(cty.String)))
	})

	t.Run("diagnostic when the path is a directory", func(t *testing.T) {
		diags := FileDiagnostic("path", cty.StringVal(dir))
		assert.True(t, diags.HasErrors())
	})

	t.Run("diagnostic when the file does not exist", func(t *testing.T) {
		diags := FileDiagnostic("path", cty.StringVal(filepath.Join(dir, "nope")))
		assert.True(t, diags.HasErrors())
	})
}

func TestWrongAttributeDiagnostic(t *testing.T) {
	diag := WrongAttributeDiagnostic("type", "foo", "bar")
	assert.Equal(t, hcl.DiagError, diag.Severity)
	assert.Contains(t, diag.Detail, "foo")
	assert.Contains(t, diag.Detail, "bar")
}

func TestWrongSizeDiagnostic(t *testing.T) {
	t.Run("nil for a null value", func(t *testing.T) {
		assert.Nil(t, WrongSizeDiagnostic("x", cty.NullVal(cty.String)))
	})

	t.Run("no diagnostic for a positive integer", func(t *testing.T) {
		assert.Empty(t, WrongSizeDiagnostic("x", cty.NumberIntVal(80)))
	})

	t.Run("diagnostic for a zero or negative integer", func(t *testing.T) {
		assert.True(t, WrongSizeDiagnostic("x", cty.NumberIntVal(0)).HasErrors())
		assert.True(t, WrongSizeDiagnostic("x", cty.NumberIntVal(-5)).HasErrors())
	})

	t.Run("diagnostic for a non-integer number", func(t *testing.T) {
		assert.True(t, WrongSizeDiagnostic("x", cty.NumberFloatVal(1.5)).HasErrors())
	})

	t.Run("no diagnostic for a valid percentage string", func(t *testing.T) {
		assert.Empty(t, WrongSizeDiagnostic("x", cty.StringVal("50%")))
		assert.Empty(t, WrongSizeDiagnostic("x", cty.StringVal("80")))
	})

	t.Run("diagnostic for an invalid string", func(t *testing.T) {
		assert.True(t, WrongSizeDiagnostic("x", cty.StringVal("big")).HasErrors())
	})

	t.Run("diagnostic for a non-string non-number type", func(t *testing.T) {
		assert.True(t, WrongSizeDiagnostic("x", cty.BoolVal(true)).HasErrors())
	})
}

func TestErrHasDiagnosticsIsError(t *testing.T) {
	assert.True(t, errors.Is(ErrHasDiagnostics, ErrHasDiagnostics))
}
