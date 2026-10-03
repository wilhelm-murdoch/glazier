package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// nested returns a profile whose window name nests depth lists.
func nested(depth int) string {
	return "session {\n  window {\n    name = " + strings.Repeat("[", depth) + `"x"` + strings.Repeat("]", depth) + "\n    pane {}\n  }\n}\n"
}

// doublingLocals returns a profile with a chain of n locals, each twice the size of the one before.
func doublingLocals(n int) string {
	var b strings.Builder
	b.WriteString("locals {\n  a0 = \"xxxxxxxx\"\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "  a%d = \"${local.a%d}${local.a%d}\"\n", i, i-1, i-1)
	}

	b.WriteString("}\nsession {\n  name = \"demo\"\n  window {\n    pane {}\n  }\n}\n")

	return b.String()
}

func TestCheckNesting(t *testing.T) {
	t.Run("accepts the limit and rejects one level more", func(t *testing.T) {
		// The session and window blocks add two levels, and the string adds one.
		assert.Empty(t, checkNesting([]byte(nested(maxNestingDepth-3)), "p.glaze"))

		diags := checkNesting([]byte(nested(maxNestingDepth-2)), "p.glaze")
		assert.True(t, diags.HasErrors())
		assert.Equal(t, "Nesting too deep", diags[0].Summary)
		assert.Equal(t, 3, diags[0].Subject.Start.Line)
	})

	t.Run("does not count brackets inside a string", func(t *testing.T) {
		src := "session {\n  name = \"" + strings.Repeat("[", 1000) + "\"\n}\n"
		assert.Empty(t, checkNesting([]byte(src), "p.glaze"))
	})

	t.Run("counts depth, not the number of brackets", func(t *testing.T) {
		src := "session {\n  name = \"x\"\n  window {\n    name = join(\"\", [" + strings.Repeat(`"a", `, 1000) + `"b"])` + "\n    pane {}\n  }\n}\n"
		assert.Empty(t, checkNesting([]byte(src), "p.glaze"))
	})

	t.Run("counts strings and interpolations inside each other", func(t *testing.T) {
		depth := maxNestingDepth / 2
		src := "session {\n  name = " + strings.Repeat(`"${`, depth) + `"x"` + strings.Repeat(`}"`, depth) + "\n}\n"
		assert.True(t, checkNesting([]byte(src), "p.glaze").HasErrors())
	})
}

func TestNewRejectsDeepNestingWithoutACrash(t *testing.T) {
	// About 100,000 levels overflow the stack of the parser, and a stack overflow cannot be recovered.
	p, diags := NewFromBytes([]byte(nested(100_000)), "p.glaze")
	assert.Nil(t, p)
	assert.True(t, diags.HasErrors())
	assert.Equal(t, "Nesting too deep", diags[0].Summary)

	p, diags = New(writeGlaze(t, nested(100_000)))
	assert.Nil(t, p)
	assert.Equal(t, "Nesting too deep", diags[0].Summary)
}

func TestVarFileRejectsDeepNesting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vars.hcl")
	assert.NoError(t, os.WriteFile(path, []byte("x = "+strings.Repeat("[", 100_000)+strings.Repeat("]", 100_000)+"\n"), 0o600))

	_, diags := loadVarFile(path, map[string]*Variable{})
	assert.True(t, diags.HasErrors())
	assert.Equal(t, "Nesting too deep", diags[0].Summary)
}

func TestValueSize(t *testing.T) {
	assert.Equal(t, 5, valueSize(cty.StringVal("hello"), maxLocalsSize))
	assert.Equal(t, 1, valueSize(cty.NumberIntVal(42), maxLocalsSize))
	assert.Equal(t, 0, valueSize(cty.NullVal(cty.String), maxLocalsSize))
	assert.Equal(t, 2+3+3, valueSize(cty.ListVal([]cty.Value{cty.StringVal("abc"), cty.StringVal("def")}), maxLocalsSize))
	assert.Equal(t, 1+1+1+2, valueSize(cty.ObjectVal(map[string]cty.Value{"a": cty.True, "b": cty.StringVal("xy")}), maxLocalsSize))

	t.Run("stops counting once it passes the limit", func(t *testing.T) {
		// Three levels of 1,000 shared elements use little memory, but a full count visits a billion elements.
		value := cty.StringVal("x")
		for range 3 {
			items := make([]cty.Value, 1000)
			for i := range items {
				items[i] = value
			}

			value = cty.ListVal(items)
		}

		start := time.Now()
		assert.Greater(t, valueSize(value, maxLocalsSize), maxLocalsSize)
		assert.Less(t, time.Since(start), time.Second)
	})
}

func TestLocalsBudget(t *testing.T) {
	t.Run("stops a doubling chain at the local that passes the budget", func(t *testing.T) {
		p, diags := New(writeGlaze(t, doublingLocals(20)))
		assert.False(t, diags.HasErrors())

		_, diags = p.VariableContext(nil, "", true)
		require.True(t, diags.HasErrors())
		assert.Equal(t, "Locals too large", diags[0].Summary)
		// 8 bytes doubled 17 times is 1 MiB, and with the locals before it the sum passes the budget at a17.
		assert.Contains(t, diags[0].Detail, "local.a17")
	})

	t.Run("also stops the lenient pass that down uses", func(t *testing.T) {
		p, diags := New(writeGlaze(t, doublingLocals(20)))
		assert.False(t, diags.HasErrors())

		_, diags = p.VariableContext(nil, "", false)
		require.True(t, diags.HasErrors())
		assert.Equal(t, "Locals too large", diags[0].Summary)
	})

	t.Run("accepts locals under the budget", func(t *testing.T) {
		p, diags := New(writeGlaze(t, doublingLocals(10)))
		assert.False(t, diags.HasErrors())

		_, diags = p.VariableContext(nil, "", true)
		assert.False(t, diags.HasErrors(), diags.Error())
	})
}
