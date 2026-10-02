package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestRandomFunction(t *testing.T) {
	random := Functions()["random"]
	require.NotNil(t, random, "random must be registered in the function library")

	options := []string{"watson", "westbrook", "heywood", "arasaka", "kabuki"}
	elems := make([]cty.Value, len(options))
	for i, o := range options {
		elems[i] = cty.StringVal(o)
	}

	t.Run("returns a member of the list", func(t *testing.T) {
		// A tuple is what for-comprehensions and bracket literals produce.
		list := cty.TupleVal(elems)

		seen := map[string]bool{}
		for range 300 {
			got, err := random.Call([]cty.Value{list})
			require.NoError(t, err)
			require.Equal(t, cty.String, got.Type())
			assert.Contains(t, options, got.AsString())
			seen[got.AsString()] = true
		}

		// Seeded randomness: across 300 draws from 5 options we must see more
		// than one distinct result (failure probability ~5·(1/5)^300).
		assert.Greater(t, len(seen), 1, "expected variety across draws")
	})

	t.Run("accepts a typed list too", func(t *testing.T) {
		got, err := random.Call([]cty.Value{cty.ListVal(elems)})
		require.NoError(t, err)
		assert.Contains(t, options, got.AsString())
	})

	t.Run("coerces non-string elements", func(t *testing.T) {
		got, err := random.Call([]cty.Value{cty.TupleVal([]cty.Value{cty.NumberIntVal(7)})})
		require.NoError(t, err)
		assert.Equal(t, "7", got.AsString())
	})

	t.Run("an empty list is an error", func(t *testing.T) {
		_, err := random.Call([]cty.Value{cty.EmptyTupleVal})
		require.Error(t, err)
	})

	t.Run("a null argument is an error", func(t *testing.T) {
		_, err := random.Call([]cty.Value{cty.NullVal(cty.List(cty.String))})
		require.Error(t, err)
	})
}

func TestRandomRejectsAMapAndAnObject(t *testing.T) {
	random := Functions()["random"]

	for _, value := range []cty.Value{
		cty.MapVal(map[string]cty.Value{"a": cty.StringVal("x")}),
		cty.ObjectVal(map[string]cty.Value{"a": cty.StringVal("x")}),
		cty.StringVal("x"),
	} {
		// Before, Index panicked on a map or an object, and the error held a Go stack dump.
		_, err := random.Call([]cty.Value{value})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "random requires a non-empty list")
		assert.NotContains(t, err.Error(), "goroutine")
	}
}

func TestRandomPicksFromASet(t *testing.T) {
	got, err := Functions()["random"].Call([]cty.Value{cty.SetVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")})})
	require.NoError(t, err)
	assert.Contains(t, []string{"a", "b"}, got.AsString())
}
