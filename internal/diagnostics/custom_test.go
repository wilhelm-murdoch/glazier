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
	assert.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))

	home := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(home, "proj"), 0o700))
	t.Setenv("HOME", home)

	for _, valid := range []string{dir, "sub", "./sub", "~", "~/proj"} {
		assert.Empty(t, DirectoryDiagnostic("starting directory", cty.StringVal(valid), dir), valid)
	}

	assert.Empty(t, DirectoryDiagnostic("starting directory", cty.NullVal(cty.String), dir))

	for _, invalid := range []string{"", file, "f.txt", filepath.Join(dir, "nope"), "nope", "~/nope"} {
		assert.True(t, DirectoryDiagnostic("starting directory", cty.StringVal(invalid), dir).HasErrors(), invalid)
	}

	t.Run("says that glaze does not expand ~user", func(t *testing.T) {
		diags := DirectoryDiagnostic("starting directory", cty.StringVal("~root/x"), dir)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags[0].Detail, "not `~user`")
	})

	t.Run("says why ~ fails without a home directory", func(t *testing.T) {
		t.Setenv("HOME", "")

		diags := DirectoryDiagnostic("starting directory", cty.StringVal("~/proj"), dir)
		assert.True(t, diags.HasErrors())
		assert.Contains(t, diags[0].Detail, "could not expand `~`")
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

func TestRequiredDiagnostic(t *testing.T) {
	assert.Empty(t, RequiredDiagnostic("direction", cty.StringVal("up")))
	assert.True(t, RequiredDiagnostic("direction", cty.NullVal(cty.String)).HasErrors())
}

func TestNoNullsDiagnostic(t *testing.T) {
	assert.Empty(t, NoNullsDiagnostic("commands", cty.ListVal([]cty.Value{cty.StringVal("ls")})))
	assert.Empty(t, NoNullsDiagnostic("commands", cty.NullVal(cty.List(cty.String))))

	diags := NoNullsDiagnostic("commands", cty.ListVal([]cty.Value{cty.StringVal("ls"), cty.NullVal(cty.String)}))
	assert.True(t, diags.HasErrors())
	assert.Contains(t, diags[0].Detail, "index 1")

	diags = NoNullsDiagnostic("envs", cty.MapVal(map[string]cty.Value{"A": cty.StringVal("1"), "B": cty.NullVal(cty.String)}))
	assert.True(t, diags.HasErrors())
	assert.Contains(t, diags[0].Detail, `key "B"`)
}

func TestHooksDiagnostic(t *testing.T) {
	hooks := func(values map[string]cty.Value) cty.Value { return cty.MapVal(values) }

	assert.Empty(t, HooksDiagnostic(hooks(map[string]cty.Value{"session-created": cty.StringVal("x"), "after-new-window[1]": cty.StringVal("y")})))
	assert.Empty(t, HooksDiagnostic(cty.NullVal(cty.Map(cty.String))))

	diags := HooksDiagnostic(hooks(map[string]cty.Value{"session-create": cty.StringVal("x"), "pane-dead": cty.StringVal("y")}))
	assert.Len(t, diags, 2, "every unknown name is reported")
	assert.Contains(t, diags[0].Detail, `"pane-dead"`)

	diags = HooksDiagnostic(hooks(map[string]cty.Value{"session-created": cty.NullVal(cty.String)}))
	assert.True(t, diags.HasErrors())
	assert.Contains(t, diags[0].Detail, "null")
}

func TestLayoutCellsDiagnostic(t *testing.T) {
	window := func(layout cty.Value, panes int) cty.Value {
		items := make([]cty.Value, panes)
		for i := range items {
			items[i] = cty.EmptyObjectVal
		}

		return cty.ObjectVal(map[string]cty.Value{"layout": layout, "panes": cty.TupleVal(items)})
	}

	twoCells := cty.StringVal("e5be,80x24,0,0{40x24,0,0,1,39x24,41,0,2}")

	assert.Empty(t, LayoutCellsDiagnostic(window(twoCells, 2)))
	assert.Empty(t, LayoutCellsDiagnostic(window(cty.StringVal("tiled"), 5)), "a preset fits any number of panes")
	assert.Empty(t, LayoutCellsDiagnostic(window(cty.NullVal(cty.String), 3)))

	for _, panes := range []int{1, 3} {
		diags := LayoutCellsDiagnostic(window(twoCells, panes))
		assert.True(t, diags.HasErrors(), panes)
		assert.Contains(t, diags[0].Detail, "describes 2 panes")
	}
}

func TestSessionNameDiagnosticRejectsAnEmptyName(t *testing.T) {
	diags := SessionNameDiagnostic(cty.StringVal(""))
	assert.True(t, diags.HasErrors())
	assert.Contains(t, diags[0].Detail, "must not be empty")
}
