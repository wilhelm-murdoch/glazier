package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// TestResolution checks the order in which glaze looks for a profile: --profile-path, then .glaze in the current directory, then $GLAZE_PATH/.glaze.
func TestResolution(t *testing.T) {
	harness.Run(t, "res_cwd", func(c *harness.Case) {
		c.Simple("from-cwd")
		c.OK(c.Up(), "profile from cwd")
		c.SessionExists("cwd session created", "from-cwd")
	})

	harness.Run(t, "res_flag", func(c *harness.Case) {
		c.Simple("from-flag", "x/prof.hcl")
		c.OK(c.Up("--profile-path", "x/prof.hcl"), "--profile-path non-.glaze name")
		c.SessionExists("flag session created", "from-flag")
	})

	harness.Run(t, "res_glaze_path", func(c *harness.Case) {
		c.Simple("from-glaze-path", "gp/.glaze")
		c.Mkdir("empty")
		c.Cd("empty")
		c.OK(c.UpWith(glazePath(c.Path("gp"))), "$GLAZE_PATH fallback")
		c.SessionExists("GLAZE_PATH session", "from-glaze-path")
	})

	harness.Run(t, "res_glaze_path_tilde", func(c *harness.Case) {
		c.Simple("from-glaze-path-tilde", "home/gp/.glaze")
		c.Mkdir("empty")
		c.Cd("empty")
		c.OK(c.UpWith(glazePath("~/gp")), "$GLAZE_PATH with ~")
		c.SessionExists("GLAZE_PATH ~ session", "from-glaze-path-tilde")
	})

	harness.Run(t, "res_precedence", func(c *harness.Case) {
		c.Simple("cwd-wins")
		c.Simple("glaze-path-loses", "gp/.glaze")
		c.OK(c.UpWith(glazePath(c.Path("gp"))), "up with .glaze in cwd and in GLAZE_PATH")
		c.SessionExists("cwd beats GLAZE_PATH", "cwd-wins")
		c.SessionGone("GLAZE_PATH not used when cwd has .glaze", "glaze-path-loses")
	})

	harness.Run(t, "res_flag_beats_cwd", func(c *harness.Case) {
		c.Simple("cwd-loses")
		c.Simple("flag-wins", "other.glaze")
		c.OK(c.Up("--profile-path", "other.glaze"), "up with --profile-path and .glaze in cwd")
		c.SessionExists("--profile-path beats cwd", "flag-wins")
		c.SessionGone("cwd ignored with --profile-path", "cwd-loses")
	})

	harness.Run(t, "res_tilde", func(c *harness.Case) {
		c.Simple("from-tilde", "home/p.glaze")
		c.OK(c.Up("--profile-path", "~/p.glaze"), "--profile-path with ~")
		c.SessionExists("tilde session", "from-tilde")
	})

	harness.Run(t, "res_missing", func(c *harness.Case) {
		c.Mkdir("empty")
		c.Cd("empty")
		r := c.Up()
		c.Fails(r, "no profile anywhere")
		c.Match("missing profile message mentions profile", `profile not found`, r.Stderr)
		c.Match("missing profile message lists each place it searched", `--profile-path(.|\n)*current directory(.|\n)*GLAZE_PATH`, r.Stderr)
		c.NoServer("missing profile")
	})

	harness.Run(t, "res_flag_missing", func(c *harness.Case) {
		r := c.Up("--profile-path", "nope.glaze")
		c.Fails(r, "--profile-path to missing file")
		c.Match("missing flag file message names the file", "`nope.glaze` does not exist", r.Stderr)
	})

	harness.Run(t, "res_dir", func(c *harness.Case) {
		c.Mkdir("adir")
		r := c.Up("--profile-path", "adir")
		c.Fails(r, "--profile-path to a directory")
		c.Match("dir profile message says it is a directory", "`adir` is a directory", r.Stderr)
	})
}

// glazePath returns the options of a command with GLAZE_PATH set to dir.
func glazePath(dir string) harness.Opts {
	return harness.Opts{Env: []string{"GLAZE_PATH=" + dir}}
}
