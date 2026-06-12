package parser

import (
	"testing"

	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

// FuzzDecode drives arbitrary bytes through the full profile pipeline — HCL
// parsing, spec validation (including the layout-string and size validators),
// and decoding into typed structs. A profile is the one attacker-controllable
// input glaze consumes, so the invariant is strict: any input either yields
// diagnostics or a decoded session, and never a panic or a hang. The seed
// corpus covers every block type, both layout forms, variable interpolation,
// and the template functions; `make test` replays it as a plain test.
func FuzzDecode(f *testing.F) {
	seeds := []string{
		``,
		`session {`,
		`session {}`,
		`session { name = "demo" }`,
		`session { name = "gig-${district}" window { pane {} } }`,
		`session { name = upper(trimspace(" demo ")) window { pane {} } }`,
		`session { name = "demo" starting_directory = "/nonexistent" }`,
		`session {
		  name = "daemon-run"
		  envs = { EDITOR = "nvim" }
		  hooks = { "session-created" = "run-shell 'echo jacked-in'" }
		  options = { "base-index" = "1" }
		  window {
		    name   = "ice-breaker"
		    layout = "main-vertical"
		    focus  = true
		    pane {
		      name     = "breach-protocol"
		      commands = ["nvim ./daemons", "echo upload ready"]
		      size { x = "60%" y = "100" }
		      adjust { direction = "left" amount = "5" }
		    }
		    pane { commands = ["watch -n1 netwatch"] }
		  }
		}`,
		`session { window { layout = "bb62,80x24,0,0" pane {} } }`,
		`session { window { layout = "e5be,80x24,0,0{40x24,0,0,1,39x24,41,0,2}" pane {} } }`,
		`session { window { layout = "not-a-layout" pane {} } }`,
		`session { window { pane { size { x = "999999999999999999999%" } } } }`,
		`session { window { pane { adjust { direction = "sideways" amount = "x" } } } }`,
		"session { name = \"\x00\" }",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	// A fixed eval context: enough variables for the interpolation seeds to
	// resolve, without the per-run environment noise of CollectVariables.
	variables := map[string]cty.Value{
		"district": cty.StringVal("watson"),
		"path": cty.ObjectVal(map[string]cty.Value{
			"base": cty.StringVal("glazier"),
			"pwd":  cty.StringVal("/tmp/glazier"),
		}),
	}

	f.Fuzz(func(t *testing.T, src string) {
		p, diags := NewFromBytes([]byte(src), "fuzz.glaze")
		if diags.HasErrors() {
			return
		}

		session, decodeDiags := p.Decode(spec.Session, BuildEvalContext(variables))
		if decodeDiags.HasErrors() {
			return
		}

		if session == nil {
			t.Fatalf("decode returned neither diagnostics nor a session for: %q", src)
		}
	})
}

// FuzzCollectVariables exercises both variable-collection paths with the same
// arbitrary input: a `--var` flag value and a `GLAZE_ENV_`-prefixed
// environment entry. Collected values must always be cty strings keyed by the
// text before the first `=`.
func FuzzCollectVariables(f *testing.F) {
	seeds := []string{
		"key=value",
		"GLAZE_ENV_district=watson",
		"novalue",
		"=",
		"a=b=c",
		"spaced key =value",
		"GLAZE_ENV_=empty",
		"\x00=\xff",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		for name, collected := range collectFlagVariables([]string{input}) {
			if collected.Type() != cty.String {
				t.Fatalf("flag variable %q collected as non-string: %#v", name, collected)
			}
		}

		for name, collected := range collectEnvVariables([]string{input}, glazeEnvPrefix) {
			if collected.Type() != cty.String {
				t.Fatalf("env variable %q collected as non-string: %#v", name, collected)
			}
		}
	})
}
