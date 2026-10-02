package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// EnvVariablePrefix marks environment variables glaze exposes to profiles
// under the env.* namespace: GLAZE_ENV_district=... becomes env.district.
const EnvVariablePrefix = "GLAZE_ENV_"

// collectBaseVariables returns the env object (GLAZE_ENV_* without the prefix) and the path object.
// Without a current directory, for example one that was deleted, there is no path object, and the error says why.
func collectBaseVariables() (map[string]cty.Value, error) {
	out := make(map[string]cty.Value)

	out["env"] = cty.ObjectVal(collectEnvVariables(os.Environ(), EnvVariablePrefix))

	pwd, err := os.Getwd()
	if err != nil {
		return out, err
	}

	// path.pwd is the working directory and path.base is its last element.
	out["path"] = cty.ObjectVal(map[string]cty.Value{
		"base": cty.StringVal(filepath.Base(pwd)),
		"pwd":  cty.StringVal(pwd),
	})

	return out, nil
}

// pathUnavailable returns an error for the first use of path.pwd or path.base, when there is no current directory.
// A profile that does not use them needs no current directory.
func (p *Parser) pathUnavailable(cause error) hcl.Diagnostics {
	body, ok := p.File.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}

	var first *hcl.Range
	_ = hclsyntax.VisitAll(body, func(node hclsyntax.Node) hcl.Diagnostics {
		if expr, ok := node.(*hclsyntax.ScopeTraversalExpr); ok && first == nil && expr.Traversal.RootName() == "path" {
			first = expr.SrcRange.Ptr()
		}

		return nil
	})

	if first == nil {
		return nil
	}

	return hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "Current directory not available",
		Detail:   fmt.Sprintf("The profile uses path.pwd or path.base, but glaze cannot read the current directory: %s. Run glaze from a directory that exists.", cause),
		Subject:  first,
	}}
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

// VariableContext builds the evaluation context: env and path, then var from --var and --var-file, then local.
// requireAll is false for `down`. The context is usable even with errors, so callers can show every diagnostic.
func (p *Parser) VariableContext(flags []string, varFile string, requireAll bool) (*hcl.EvalContext, hcl.Diagnostics) {
	base, err := collectBaseVariables()
	if err != nil {
		if diags := p.pathUnavailable(err); diags.HasErrors() {
			return nil, diags
		}
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
