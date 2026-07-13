package parser

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// EnvVariablePrefix marks environment variables glaze exposes to profiles
// under the env.* namespace: GLAZE_ENV_district=... becomes env.district.
const EnvVariablePrefix = "GLAZE_ENV_"

// collectBaseVariables returns the always-available top-level variables: the
// `env` object holding any GLAZE_ENV_* entries (with the prefix stripped) and
// the built-in `path` object. Each lives in its own namespace, mirroring how
// declared variables are exposed under `var` and locals under `local`;
// nothing sits bare at the root.
func collectBaseVariables() (map[string]cty.Value, error) {
	out := make(map[string]cty.Value)

	out["env"] = cty.ObjectVal(collectEnvVariables(os.Environ(), EnvVariablePrefix))

	defaults, err := addDefaultVariables()
	if err != nil {
		return nil, err
	}
	maps.Copy(out, defaults)

	return out, nil
}

// collectEnvVariables parses environment variables that start with prefix.
func collectEnvVariables(envs []string, prefix string) map[string]cty.Value {
	out := make(map[string]cty.Value)

	for _, env := range envs {
		if !strings.HasPrefix(env, prefix) {
			continue
		}

		key, value, ok := strings.Cut(strings.TrimPrefix(env, prefix), "=")
		if !ok || key == "" {
			continue
		}

		out[key] = cty.StringVal(value)
	}

	return out
}

// addDefaultVariables appends the built-in path object: path.pwd (the
// working directory) and path.base (its basename).
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

// VariableContext builds the evaluation context for a profile, namespace by
// namespace: the built-ins (env.*, path.*), the declared `var` object
// resolved from the --var flags and --var-file, and finally the `local`
// object last, so locals can reference everything before them. requireAll
// enforces that every declared variable (and local) resolves; `down` passes
// false because it evaluates only the session name and must not demand
// variables used solely deeper in the profile. The returned context is
// always usable even when diagnostics contain errors, so callers can render
// the full set before deciding to halt.
func (p *Parser) VariableContext(flags []string, varFile string, requireAll bool) (*hcl.EvalContext, hcl.Diagnostics) {
	base, err := collectBaseVariables()
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Could not collect variables",
			Detail:   err.Error(),
		}}
	}

	declared, diags := p.DecodeVariableBlocks()

	resolved, resolveDiags := ResolveVariables(declared, flags, varFile, requireAll)
	diags = diags.Extend(resolveDiags)

	base["var"] = cty.ObjectVal(resolved)

	locals, localDiags := p.resolveLocals(base, requireAll)
	diags = diags.Extend(localDiags)

	base["local"] = cty.ObjectVal(locals)

	return BuildEvalContext(base), diags
}
