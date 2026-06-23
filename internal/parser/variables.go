package parser

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

const glazeEnvPrefix = "GLAZE_ENV_"

// collectBaseVariables returns the always-available top-level variables: the
// `env` object holding any GLAZE_ENV_* entries (with the prefix stripped) and
// the built-in `path` object. Each lives in its own namespace, mirroring how
// declared variables are exposed under `var`; nothing sits bare at the root.
// Declared variables are deliberately not collected here: they are resolved
// against their `variable` blocks and merged under `var` by VariableContext.
func collectBaseVariables() (map[string]cty.Value, error) {
	out := make(map[string]cty.Value)

	out["env"] = cty.ObjectVal(collectEnvVariables(os.Environ(), glazeEnvPrefix))

	defaults, err := addDefaultVariables()
	if err != nil {
		return nil, err
	}
	maps.Copy(out, defaults)

	return out, nil
}

// collectEnvVariables parses environment variables that start with the `glazeEnvPrefix` prefix.
func collectEnvVariables(envs []string, prefix string) map[string]cty.Value {
	out := make(map[string]cty.Value)

	for _, env := range envs {
		if !strings.HasPrefix(env, prefix) {
			continue
		}

		trimmed := strings.TrimPrefix(env, prefix)

		if !strings.Contains(trimmed, "=") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		out[parts[0]] = cty.StringVal(parts[1])
	}

	return out
}

// collectFlagVariables parses variables passed via command line with multiple `--var` flags.
func collectFlagVariables(vars []string) map[string]cty.Value {
	out := make(map[string]cty.Value)

	for _, flag := range vars {
		if !strings.Contains(flag, "=") {
			continue
		}

		parts := strings.SplitN(flag, "=", 2)
		out[strings.TrimSpace(parts[0])] = cty.StringVal(parts[1])
	}

	return out
}

// addDefaultVariables is used to append other useful variables for use within a glaze definition file.
func addDefaultVariables() (map[string]cty.Value, error) {
	out := make(map[string]cty.Value)

	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not read current working directory: %w", err)
	}

	out["path"] = cty.ObjectVal(map[string]cty.Value{
		"base": cty.StringVal(filepath.Base(pwd)),
		"pwd":  cty.StringVal(pwd),
	})

	return out, nil
}

// BuildEvalContext returns an `EvalContext` which is used to pass the key value map of
// variables sourced from CollectVariables as well as useful string template functions
// for use within a glaze definition file.
func BuildEvalContext(variables map[string]cty.Value) *hcl.EvalContext {
	return &hcl.EvalContext{
		Variables: variables,
		Functions: map[string]function.Function{
			"replace":      stdlib.ReplaceFunc,
			"regexreplace": stdlib.RegexReplaceFunc,
			"upper":        stdlib.UpperFunc,
			"lower":        stdlib.LowerFunc,
			"reverse":      stdlib.ReverseFunc,
			"len":          stdlib.LengthFunc,
			"substr":       stdlib.SubstrFunc,
			"join":         stdlib.JoinFunc,
			"title":        stdlib.TitleFunc,
			"trim":         stdlib.TrimFunc,
			"trimspace":    stdlib.TrimSpaceFunc,
			"trimsuffix":   stdlib.TrimSuffixFunc,
			"trimprefix":   stdlib.TrimPrefixFunc,
			"chomp":        stdlib.ChompFunc,
		},
	}
}
