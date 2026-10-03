package cases

import (
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// down kills the session of the profile, or the one that --session names.
func TestDown(t *testing.T) {
	harness.Run(t, "down_basic", func(c *harness.Case) {
		c.Simple("dn")
		c.TmuxSetup("new-session", "-d", "-s", "other")
		c.OK(c.Up(), "up")
		c.OK(c.Down(), "down")
		c.SessionGone("down kills profile session", "dn")
		c.SessionExists("down leaves other sessions", "other")
		c.OK(c.Down(), "down again (not running) is a no-op")
	})

	harness.Run(t, "down_no_server", func(c *harness.Case) {
		c.Simple("dn")
		r := c.Down()
		c.OK(r, "down with no tmux server is a no-op")
		c.Match("down with no server says the session is not running", `nothing to do; session is not running session=dn`, r.Stderr)
	})

	// The case runs down from an empty directory, so glaze finds no profile and uses --session only.
	harness.Run(t, "down_session_flag", func(c *harness.Case) {
		c.Mkdir("empty")
		c.Cd("empty")
		c.TmuxSetup("new-session", "-d", "-s", "tgt")
		c.TmuxSetup("new-session", "-d", "-s", "keep")
		c.OK(c.Down("--session", "tgt"), "down --session without profile")
		c.SessionGone("tgt killed", "tgt")
		c.SessionExists("keep survives", "keep")
		c.OK(c.Down("--session", "nonexistent"), "down --session unknown is a no-op")
		c.TmuxSetup("new-session", "-d", "-s", "sp ace")
		c.OK(c.Down("--session", "sp ace"), "down --session with space")
		c.SessionGone("spaced session killed", "sp ace")
	})

	// tmux matches a session name by prefix unless the target starts with =.
	harness.Run(t, "down_prefix_match", func(c *harness.Case) {
		c.Mkdir("empty")
		c.Cd("empty")
		c.TmuxSetup("new-session", "-d", "-s", "project-long")
		c.OK(c.Down("--session", "project"), "down --session prefix")
		c.SessionExists("prefix does not kill project-long", "project-long")
	})

	harness.Run(t, "down_name_number", func(c *harness.Case) {
		c.Fixture("down/number-name.glaze")
		c.OK(c.Up(), "up with a number as the session name")
		c.SessionExists("up creates session 42", "42")
		c.OK(c.Down(), "down with a number as the session name")
		c.SessionGone("down kills session 42", "42")
	})

	harness.Run(t, "down_name_missing", func(c *harness.Case) {
		c.Fixture("structure/no-names.glaze")
		c.OK(c.Up(), "up without a session name")
		c.SessionExists("up creates session default", "default")
		c.OK(c.Down(), "down without a session name")
		c.SessionGone("down kills session default", "default")
	})

	harness.Run(t, "down_name_random", func(c *harness.Case) {
		c.Fixture("down/random-name.glaze")
		r := c.Up()
		c.Fails(r, "up rejects random() in the session name")
		c.NoServer("random() in the session name")
		c.Match("the error says why", "must not use random", r.Stderr)
		c.Fails(c.Down(), "down rejects random() in the session name")
	})

	harness.Run(t, "down_vars", func(c *harness.Case) {
		c.Fixture("variables/typed.glaze")
		c.Must(c.Up("--var", "fixer=x", "--var", "district=pacifica"), "up with --var")
		c.SessionExists("up", "gig-pacifica")
		// Without --var, down computes the name from the default and needs no value for fixer.
		r := c.Down()
		c.OK(r, "down without --var when name uses a default var")
		c.Match("down without --var looks for the default name", `session is not running session=gig-watson`, r.Stderr)
		c.SessionExists("default-name down leaves pacifica", "gig-pacifica")
		c.OK(c.Down("--var", "district=pacifica"), "down with --var (deep required var missing)")
		c.SessionGone("interpolated session killed", "gig-pacifica")
	})
}
