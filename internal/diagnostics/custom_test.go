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

func TestNameDiagnostic(t *testing.T) {
	t.Run("warns when tmux would rewrite a window name", func(t *testing.T) {
		diags := NameDiagnostic("window", cty.StringVal(`w\z`))
		if assert.Len(t, diags, 1) {
			assert.Equal(t, hcl.DiagWarning, diags[0].Severity)
			assert.Equal(t, "Window name will be changed", diags[0].Summary)
			assert.Contains(t, diags[0].Detail, `"w-z"`)
		}
	})

	t.Run("accepts characters that only session names cannot use", func(t *testing.T) {
		assert.Empty(t, NameDiagnostic("pane", cty.StringVal("p.$x:1")))
	})
}

func TestSessionNameDiagnostic(t *testing.T) {
	t.Run("warns when tmux would rewrite the name", func(t *testing.T) {
		diags := SessionNameDiagnostic(cty.StringVal("a.b"))
		if assert.Len(t, diags, 1) {
			assert.Equal(t, hcl.DiagWarning, diags[0].Severity)
			assert.Contains(t, diags[0].Detail, `"a-b"`)
		}
	})

	t.Run("accepts a name tmux keeps", func(t *testing.T) {
		assert.Empty(t, SessionNameDiagnostic(cty.StringVal("my session;1")))
	})

	t.Run("ignores null and unknown values", func(t *testing.T) {
		assert.Empty(t, SessionNameDiagnostic(cty.NullVal(cty.String)))
		assert.Empty(t, SessionNameDiagnostic(cty.UnknownVal(cty.String)))
	})
}

func TestDirectoryDiagnostic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	assert.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

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

func TestSizeDiagnostic(t *testing.T) {
	assert.Nil(t, SizeDiagnostic("x", cty.NullVal(cty.String)))

	for _, valid := range []string{"1", "80", "5000", "1%", "50%", "100%"} {
		assert.Empty(t, SizeDiagnostic("x", cty.StringVal(valid)), valid)
	}

	for _, invalid := range []string{"0", "0%", "101%", "-5", "big", "", " 10", "10 %", "10.5", "1e3", "%", "99999999999999999999"} {
		assert.True(t, SizeDiagnostic("x", cty.StringVal(invalid)).HasErrors(), invalid)
	}
}

func TestAmountDiagnostic(t *testing.T) {
	assert.Nil(t, AmountDiagnostic("amount", cty.NullVal(cty.String)))

	for _, valid := range []string{"1", "5", "200"} {
		assert.Empty(t, AmountDiagnostic("amount", cty.StringVal(valid)), valid)
	}

	for _, invalid := range []string{"0", "-3", "10%", "100%", "abc", ""} {
		diags := AmountDiagnostic("amount", cty.StringVal(invalid))
		assert.True(t, diags.HasErrors(), invalid)
	}

	assert.Contains(t, AmountDiagnostic("amount", cty.StringVal("10%"))[0].Detail, "A percentage is not supported")
}

func TestErrHasDiagnosticsIsError(t *testing.T) {
	assert.True(t, errors.Is(ErrHasDiagnostics, ErrHasDiagnostics))
}
