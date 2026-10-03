package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// Profile resolution: --profile-path, then .glaze in the current directory, then $GLAZE_PATH/.glaze.
func TestResolution(t *testing.T) {
	harness.Run(t, "res_cwd", func(c *harness.Case) {
		c.Simple("rcwd")
		c.OK(c.Up(), "profile from cwd")
		c.SessionExists("cwd session created", "rcwd")
	})

	harness.Run(t, "res_flag", func(c *harness.Case) {
		c.Simple("rflag", "x/prof.hcl")
		c.OK(c.Up("--profile-path", "x/prof.hcl"), "--profile-path non-.glaze name")
		c.SessionExists("flag session created", "rflag")
	})

	harness.Run(t, "res_glaze_path", func(c *harness.Case) {
		c.Simple("rgp", "gp/.glaze")
		c.Mkdir("empty")
		c.Cd("empty")
		c.OK(c.UpWith(harness.Opts{Env: []string{"GLAZE_PATH=" + c.Path("gp")}}), "$GLAZE_PATH fallback")
		c.SessionExists("GLAZE_PATH session", "rgp")
	})

	harness.Run(t, "res_glaze_path_tilde", func(c *harness.Case) {
		c.Simple("rgpt", "home/gp/.glaze")
		c.Mkdir("empty")
		c.Cd("empty")
		c.OK(c.UpWith(harness.Opts{Env: []string{"GLAZE_PATH=~/gp"}}), "$GLAZE_PATH with ~")
		c.SessionExists("GLAZE_PATH ~ session", "rgpt")
	})

	harness.Run(t, "res_precedence", func(c *harness.Case) {
		c.Simple("rcwdwins")
		c.Simple("rgploses", "gp/.glaze")
		c.UpWith(harness.Opts{Env: []string{"GLAZE_PATH=" + c.Path("gp")}})
		c.SessionExists("cwd beats GLAZE_PATH", "rcwdwins")
		c.SessionGone("GLAZE_PATH not used when cwd has .glaze", "rgploses")
	})

	harness.Run(t, "res_flag_beats_cwd", func(c *harness.Case) {
		c.Simple("rcwd2")
		c.Simple("rflag2", "other.glaze")
		c.Up("--profile-path", "other.glaze")
		c.SessionExists("--profile-path beats cwd", "rflag2")
		c.SessionGone("cwd ignored with --profile-path", "rcwd2")
	})

	harness.Run(t, "res_tilde", func(c *harness.Case) {
		c.Simple("rtilde", "home/p.glaze")
		c.OK(c.Up("--profile-path", "~/p.glaze"), "--profile-path with ~")
		c.SessionExists("tilde session", "rtilde")
	})

	harness.Run(t, "res_missing", func(c *harness.Case) {
		c.Mkdir("empty")
		c.Cd("empty")
		r := c.Up()
		c.Fails(r, "no profile anywhere")
		c.Match("missing profile message mentions profile", `[Pp]rofile|glaze|[Nn]ot found|[Nn]o such`, r.Output())
		c.Match("missing profile message lists each place it searched", `--profile-path(.|\n)*current directory(.|\n)*GLAZE_PATH`, r.Output())
		c.NoServer("missing profile")
	})

	harness.Run(t, "res_flag_missing", func(c *harness.Case) {
		r := c.Up("--profile-path", "nope.glaze")
		c.Fails(r, "--profile-path to missing file")
		c.Match("missing flag file message names the file", "`nope.glaze` does not exist", r.Output())
	})

	harness.Run(t, "res_dir", func(c *harness.Case) {
		c.Mkdir("adir")
		r := c.Up("--profile-path", "adir")
		c.Fails(r, "--profile-path to a directory")
		c.Match("dir profile message says it is a directory", "`adir` is a directory", r.Output())
	})
}
