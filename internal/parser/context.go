package parser

import (
	"math/rand/v2"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// BuildEvalContext returns the EvalContext every profile expression is
// evaluated in: the given variable namespaces (var.*, local.*, env.*,
// path.*) plus the shared string/collection function library. The namespace
// map is assembled by VariableContext; this only wraps it.
func BuildEvalContext(variables map[string]cty.Value) *hcl.EvalContext {
	return &hcl.EvalContext{
		Variables: variables,
		Functions: Functions(),
	}
}

// Functions is the expression function library available in every profile
// expression.
func Functions() map[string]function.Function {
	return map[string]function.Function{
		"chomp":        stdlib.ChompFunc,
		"coalesce":     stdlib.CoalesceFunc,
		"concat":       stdlib.ConcatFunc,
		"csvdecode":    stdlib.CSVDecodeFunc,
		"format":       stdlib.FormatFunc,
		"join":         stdlib.JoinFunc,
		"jsondecode":   stdlib.JSONDecodeFunc,
		"len":          stdlib.LengthFunc,
		"lower":        stdlib.LowerFunc,
		"regexreplace": stdlib.RegexReplaceFunc,
		"replace":      stdlib.ReplaceFunc,
		"reverse":      stdlib.ReverseFunc,
		"split":        stdlib.SplitFunc,
		"strlen":       stdlib.StrlenFunc,
		"substr":       stdlib.SubstrFunc,
		"title":        stdlib.TitleFunc,
		"trim":         stdlib.TrimFunc,
		"trimprefix":   stdlib.TrimPrefixFunc,
		"trimspace":    stdlib.TrimSpaceFunc,
		"trimsuffix":   stdlib.TrimSuffixFunc,
		"upper":        stdlib.UpperFunc,
		"random":       randomFunc,
	}
}

// randomFunc returns a uniformly random element of the given list, coerced
// to a string. The list is typically built inline - a locals list or a
// for-comprehension - so pairing it with those is the point. math/rand/v2's
// top-level source is seeded from the runtime at process start, so results
// vary between runs without any manual seeding. An empty list is an error.
var randomFunc = function.New(&function.Spec{
	Description: "Returns a uniformly random element of the given list, as a string.",
	Params: []function.Parameter{{
		Name: "list",
		Type: cty.DynamicPseudoType,
	}},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		list := args[0]
		if list.IsNull() || !list.CanIterateElements() {
			return cty.NilVal, function.NewArgErrorf(0, "random requires a non-empty list")
		}

		length := list.LengthInt()
		if length == 0 {
			return cty.NilVal, function.NewArgErrorf(0, "random requires a non-empty list")
		}

		// The choice is cosmetic (picking a session name flourish), not a
		// security decision, so math/rand/v2 is the right generator.
		choice := list.Index(cty.NumberIntVal(int64(rand.IntN(length)))) //nolint:gosec // G404

		result, err := convert.Convert(choice, cty.String)
		if err != nil {
			return cty.NilVal, function.NewArgErrorf(0, "random list elements must be strings: %s", err)
		}

		return result, nil
	},
})
