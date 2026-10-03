package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// Variables, locals, functions and the env and path namespaces.
func TestVariables(t *testing.T) {
	harness.Run(t, "var_default_and_flag", func(c *harness.Case) {
		c.Fixture("variables/typed.glaze")
		c.OK(c.Up("--var", "fixer=wakako"), "up with required var")
		c.SessionExists("default applied", "gig-watson")
		c.Equal("flag var used", "wakako-2-false", c.WindowNames("gig-watson"))
		c.OK(c.Up("--var", "fixer=rogue", "--var", "district=arasaka", "--var", "count=7", "--var", "loud=true"), "override all")
		c.Equal("overrides applied", "rogue-7-true", c.WindowNames("gig-arasaka"))
	})

	harness.Run(t, "var_required_missing", func(c *harness.Case) {
		c.Fixture("variables/typed.glaze")
		r := c.Up()
		c.Fails(r, "missing required var")
		c.NoServer("missing var")
		c.Match("missing var names the variable", "fixer", r.Output())
	})

	harness.Run(t, "var_undeclared", func(c *harness.Case) {
		c.Fixture("variables/typed.glaze")
		c.Fails(c.Up("--var", "fixer=x", "--var", "nope=1"), "undeclared --var rejected")
		c.NoServer("undeclared var")
	})

	harness.Run(t, "var_type_errors", func(c *harness.Case) {
		c.Fixture("variables/typed.glaze")
		c.Fails(c.Up("--var", "fixer=x", "--var", "count=two"), "number var given 'two'")
		c.Fails(c.Up("--var", "fixer=x", "--var", "loud=yes"), "bool var given 'yes'")
		c.NoServer("type errors")
		// An HCL number is not only an integer.
		c.OK(c.Up("--var", "fixer=x", "--var", "count=1.5"), "number var given 1.5")
		c.Equal("number var keeps 1.5", "x-1.5-false", c.WindowNames("gig-watson"))
	})

	harness.Run(t, "var_flag_formats", func(c *harness.Case) {
		c.Fixture("variables/typed.glaze")
		c.OK(c.Up("--var", "fixer=a,b,c"), "comma in value")
		c.Equal("comma value kept whole", "a,b,c-2-false", c.WindowNames("gig-watson"))
		c.KillServer()
		c.OK(c.Up("--var", "fixer=k=v"), "= in value")
		c.Equal("= in value kept", "k=v-2-false", c.WindowNames("gig-watson"))
		c.KillServer()
		// The empty value gives the window name "-2-false", which tmux reads as a flag.
		r := c.Up("--var", "fixer=")
		c.Logf("empty value for required var: exit %d, windows %q", r.Code, c.WindowNames("gig-watson"))
		c.KillServer()
		c.Fails(c.Up("--var", "fixer"), "--var without =")
		c.Fails(c.Up("--var", "fixer =x"), "--var name with trailing space")
		c.Fails(c.Up("--var", "=x"), "--var with empty name")
	})

	harness.Run(t, "var_file", func(c *harness.Case) {
		c.Fixture("variables/typed.glaze")
		c.Write("vars.hcl", "fixer = \"fromfile\"\ndistrict = \"file\"\n")
		c.OK(c.Up("--var-file", "vars.hcl"), "--var-file HCL")
		c.SessionExists("var-file beats default", "gig-file")
		c.Equal("var-file value", "fromfile-2-false", c.WindowNames("gig-file"))
		c.OK(c.Up("--var-file", "vars.hcl", "--var", "district=flag"), "--var-file plus --var")
		c.SessionExists("--var beats var-file", "gig-flag")
		c.KillServer()
		// SPEC.md says that glaze does not support a JSON var file.
		c.Write("vars.json", "{\"fixer\": \"json\", \"district\": \"json\"}\n")
		c.ExitCode(c.Up("--var-file", "vars.json"), "--var-file JSON is an invalid profile (exit 3)", 3)
		c.NoServer("--var-file JSON")
		c.Fails(c.Up("--var-file", "missing.hcl", "--var", "fixer=x"), "missing --var-file")
		c.Write("bad.hcl", "fixer = \"x\"\nbogus = \"y\"\n")
		c.Fails(c.Up("--var-file", "bad.hcl"), "undeclared name in var-file")
		c.Write("broken.hcl", "fixer = \n")
		c.Fails(c.Up("--var-file", "broken.hcl"), "syntax error in var-file")
	})

	harness.Run(t, "var_env_namespace", func(c *harness.Case) {
		c.Fixture("variables/env.glaze")
		c.OK(c.UpWith(harness.Opts{Env: []string{"GLAZE_ENV_tok=abc123"}}), "env.* from GLAZE_ENV_")
		c.SessionExists("env value used", "e-abc123")
		c.KillServer()
		c.Fails(c.Up(), "missing env.* reference")
		c.NoServer("missing env")
		// The prefix goes, the rest of the name stays as it is.
		c.ExitCode(c.UpWith(harness.Opts{Env: []string{"GLAZE_ENV_TOK=upper"}}), "GLAZE_ENV_ names are case sensitive (exit 3)", 3)
	})

	harness.Run(t, "var_path_namespace", func(c *harness.Case) {
		c.Fixture("variables/path.glaze", "My Proj/.glaze")
		c.Cd("My Proj")
		c.OK(c.Up(), "path.pwd/path.base")
		c.Equal("path.base", "MY PROJ", c.WindowNames("pp"))
		c.Equal("path.pwd", c.Path("My Proj"), c.PanePaths("=pp:"))
	})

	harness.Run(t, "locals_and_functions", func(c *harness.Case) {
		c.Fixture("variables/locals-functions.glaze")
		c.OK(c.Up(), "up with locals and functions")
		c.SessionExists("chained locals, order independent", "gig-night-city")
		c.Match("random picks from list", `^(NVIM|HX|VIM)$`, c.WindowNames("gig-night-city"))
		c.Equal("function results", "A B|3|4|nvim+hx+vim|x,bCd", c.PaneTitles("=gig-night-city:"))
	})

	harness.Run(t, "fn_reverselist", func(c *harness.Case) {
		c.Fixture("variables/reverselist.glaze")
		c.OK(c.Up(), "up with reverselist")
		c.Equal("reverselist reverses a list", "c+b+a", c.WindowNames("rl"))
		c.Equal("reverse reverses a string", "cba", c.PaneTitles("=rl:"))
	})

	harness.Run(t, "locals_errors", func(c *harness.Case) {
		c.Fixture("variables/locals-cycle.glaze")
		c.Fails(c.Up(), "circular locals")
		c.NoServer("circular locals")
		c.Fixture("variables/locals-duplicate.glaze")
		c.Fails(c.Up(), "duplicate local")
		c.Fixture("variables/random-empty.glaze")
		c.Fails(c.Up(), "random of empty list")
		c.Fixture("variables/unknown-function.glaze")
		c.Fails(c.Up(), "unknown function")
		c.Fixture("variables/variable-duplicate.glaze")
		c.Fails(c.Up(), "duplicate variable")
	})

	harness.Run(t, "var_in_commands_hooks", func(c *harness.Case) {
		c.Fixture("variables/command.glaze")
		c.OK(c.Up("--var", "msg=wired"), "var in command")
		c.EventuallyEqual("var interpolated into command", "wired", func() string { return c.Lines("o") })
	})
}
