package parser

import (
	"math/rand/v2"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// randomFunc returns a random element of a list as a string, and an error for an empty list.
// math/rand/v2 seeds itself at start, so the result differs between runs.
var randomFunc = function.New(&function.Spec{
	Description: "Returns a uniformly random element of the given list, as a string.",
	Params: []function.Parameter{{
		Name: "list",
		Type: cty.DynamicPseudoType,
	}},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		// A map or an object can iterate too, but it is not a list.
		list := args[0]
		ty := list.Type()
		if list.IsNull() || !list.IsKnown() || (!ty.IsListType() && !ty.IsTupleType() && !ty.IsSetType()) {
			return cty.NilVal, function.NewArgErrorf(0, "random requires a non-empty list. The value has the type %s", ty.FriendlyName())
		}

		length := list.LengthInt()
		if length == 0 {
			return cty.NilVal, function.NewArgErrorf(0, "random requires a non-empty list")
		}

		// The choice is cosmetic, for example a window name, not a security decision, so math/rand/v2 is the right generator.
		// A set has no index, so walk to the chosen element.
		var choice cty.Value
		it := list.ElementIterator()
		for range rand.IntN(length) + 1 { //nolint:gosec // G404
			it.Next()
		}
		_, choice = it.Element()

		result, err := convert.Convert(choice, cty.String)
		if err != nil {
			return cty.NilVal, function.NewArgErrorf(0, "random list elements must be strings: %s", err)
		}

		return result, nil
	},
})

// BuildEvalContext returns the context for every profile expression: the namespaces from VariableContext and the functions.
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
