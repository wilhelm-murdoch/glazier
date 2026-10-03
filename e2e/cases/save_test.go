package cases

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

const (
	// unnamedWindow and unnamedPane are the formats of State without names and window indexes,
	// for a session that tmux named and save leaves without names.
	unnamedWindow = "W|#{window_active}|#{window_panes}"
	unnamedPane   = "  P #{pane_index}|#{pane_active}|#{pane_current_path}|#{pane_left},#{pane_top},#{pane_width}x#{pane_height}"

	// escapedWindow and escapedPanes are the names in save/escaping.glaze as tmux stores them.
	escapedWindow = `q"uote`
	escapedPanes  = "${dollar},%{pct},back-slash"
)

// saveRawLayout matches the layout line that save writes, so the golden file holds "RAW" in its place.
// The geometry of a preset differs between tmux versions.
var saveRawLayout = regexp.MustCompile(`(?m)^(\s*layout\s*=\s*)"(?:[^"\\]|\\.)*"$`)

// save writes a profile of a running session, without commands, envs, hooks and options.
func TestSave(t *testing.T) {
	harness.Run(t, "save_roundtrip", func(c *harness.Case) {
		c.Mkdir("d/a", "d/b")
		c.Fixture("save/rich.glaze")
		c.OK(c.Up(), "up rich profile")
		c.WaitForPanes("save-rich")
		before := c.State("save-rich", harness.WindowState, harness.PaneState)
		r := c.Save("--session", "save-rich", "--profile-path", "saved.glaze")
		c.OK(r, "save to file")
		c.NoMatch("save shows no EXPERIMENTAL warning", "EXPERIMENTAL", r.Stderr)
		saved := c.Read("saved.glaze")
		c.Logf("saved profile:\n%s", saved)
		c.OK(c.Glaze("format", "--validate", "--profile-path", "saved.glaze"), "saved profile validates")
		c.Must(c.Down(), "down")
		c.OK(c.Up("--profile-path", "saved.glaze"), "up from saved profile")
		roundTrip := func() string { return c.State("save-rich", harness.WindowState, harness.PaneState) }
		if !c.EventuallyEqual("round-trip structure, focus, paths and geometry", before, roundTrip) {
			c.Snapshot("save-rich")
		}

		for _, excluded := range []string{"commands", "envs", "hooks", "options", "hunter2", "SECRET"} {
			c.NoMatch(fmt.Sprintf("save excludes %s", excluded), excluded, saved)
		}
	})

	harness.Run(t, "save_stdout_clean", func(c *harness.Case) {
		c.Mkdir("d/a", "d/b")
		c.Fixture("save/rich.glaze")
		c.Must(c.Up(), "up")
		c.WaitForPanes("save-rich")
		r := c.Save("--session", "save-rich", "--stdout")
		c.OK(r, "save --stdout")
		c.NoMatch("save --stdout has no log lines on stdout", "INF|WRN|EXPERIMENTAL", r.Stdout)
		c.Match("save --stdout emits HCL", `^session \{`, r.Stdout)
		c.Golden("save --stdout writes the declared structure", "save/rich.glaze", saveRawLayout.ReplaceAllString(r.Stdout, `${1}"RAW"`))
		c.Equal("save --stdout writes no file", ".glaze", workFiles(c, "*glaze*"))
		c.Write("piped.glaze", r.Stdout)
		c.OK(c.Glaze("format", "--validate", "--profile-path", "piped.glaze"), "piped save output validates")
	})

	harness.Run(t, "save_default_overwrite", func(c *harness.Case) {
		c.Simple("ow")
		c.Write(".glaze", c.Read(".glaze")+"# my hand-written profile\n")
		c.Must(c.Up(), "up")
		before := c.Read(".glaze")
		r := c.Save("--session", "ow")
		c.Fails(r, "save refuses an existing .glaze")
		c.Match("save refusal says to use --force", "use --force", r.Stderr)
		c.Equal("save does not clobber existing .glaze", before, c.Read(".glaze"))
		c.OK(c.Save("--session", "ow", "--force"), "save --force replaces .glaze")
		// The hand-written profile has no focus attribute, and save always writes one.
		c.Match("save --force wrote the session", `focus += true`, c.Read(".glaze"))
	})

	harness.Run(t, "save_force_symlink", func(c *harness.Case) {
		c.Simple("sfs", "real.glaze")
		c.Must(c.Up("--profile-path", "real.glaze"), "up")
		c.Chmod("real.glaze", 0o600)
		c.Symlink("real.glaze", ".glaze")
		c.OK(c.Save("--session", "sfs", "--force"), "save --force through a symlink")
		c.True("save keeps the symlink", isSymlink(c, ".glaze"), "save replaced the symlink with a regular file")
		c.Equal("save keeps the permissions", "-rw-------", modeOf(c, "real.glaze"))
		c.Match("save wrote through the symlink", `focus += true`, c.Read("real.glaze"))
	})

	harness.Run(t, "save_outside_tmux", func(c *harness.Case) {
		c.Simple("so")
		c.Must(c.Up(), "up")
		r := c.Save("--stdout")
		c.Fails(r, "save without --session outside tmux")
		c.Match("save outside tmux asks for --session", "--session", r.Stderr)
		c.Equal("save outside tmux writes no profile", "", r.Stdout)
	})

	harness.Run(t, "save_missing_session", func(c *harness.Case) {
		c.Simple("sm")
		c.Must(c.Up(), "up")
		c.Fails(c.Save("--session", "nope", "--stdout"), "save of unknown session")
	})

	// tmux matches a session name by prefix unless the target starts with =.
	harness.Run(t, "save_prefix_session", func(c *harness.Case) {
		c.TmuxSetup("new-session", "-d", "-s", "project-long")
		r := c.Save("--session", "project", "--stdout")
		c.True("save --session does not prefix-match", r.Failed(), "saved %q", firstLine(r.Stdout))
	})

	harness.Run(t, "save_no_server", func(c *harness.Case) {
		c.Fails(c.Save("--session", "x", "--stdout"), "save with no server")
	})

	// A session that glaze did not make has tmux names, so the round trip compares everything but the names.
	harness.Run(t, "save_foreign_session", func(c *harness.Case) {
		c.Mkdir("fdir")
		c.TmuxSetup("new-session", "-d", "-s", "raw", "-c", c.Path("fdir"))
		c.TmuxSetup("split-window", "-h", "-t", "=raw:")
		c.TmuxSetup("new-window", "-t", "=raw:", "-n", "two")
		c.WaitForPanes("raw")
		before := c.State("raw", unnamedWindow, unnamedPane)
		c.OK(c.Save("--session", "raw", "--profile-path", "raw.glaze"), "save non-glaze session")
		c.Logf("saved profile:\n%s", c.Read("raw.glaze"))
		c.TmuxSetup("kill-session", "-t", "=raw")
		c.OK(c.Up("--profile-path", "raw.glaze"), "up from foreign save")
		roundTrip := func() string { return c.State("raw", unnamedWindow, unnamedPane) }
		if !c.EventuallyEqual("foreign round-trip (ignoring titles)", before, roundTrip) {
			c.Snapshot("raw")
		}
	})

	harness.Run(t, "save_escaping", func(c *harness.Case) {
		c.Fixture("save/escaping.glaze")
		c.OK(c.Up(), "up with escape-worthy names")
		c.Equal("window name literal", escapedWindow, c.WindowNames("escaping"))
		c.Equal("pane names literal", escapedPanes, c.PaneTitles("=escaping:"))
		c.OK(c.Save("--session", "escaping", "--profile-path", "s.glaze"), "save escape-worthy names")
		c.Logf("saved profile:\n%s", c.Read("s.glaze"))
		c.Must(c.Down(), "down")
		c.OK(c.Up("--profile-path", "s.glaze"), "up from saved escaped profile")
		c.Equal("escaped window name round-trips", escapedWindow, c.WindowNames("escaping"))
		c.Equal("escaped pane names round-trip", escapedPanes, c.PaneTitles("=escaping:"))
	})

	// glaze does not keep a pane title that a program in the pane changes.
	harness.Run(t, "save_titles_changed", func(c *harness.Case) {
		c.Fixture("save/title-command.glaze")
		c.Must(c.Up(), "up")
		c.EventuallyEqual("pane title after app sets it", "changed-by-app", func() string { return c.PaneTitles("=title-command:w") })
	})

	harness.Run(t, "save_deleted_dir", func(c *harness.Case) {
		c.Mkdir("gonedir")
		c.TmuxSetup("new-session", "-d", "-s", "dd", "-c", c.Path("gonedir"))
		// The shell must be in the directory before the case deletes it.
		c.WaitForPanes("dd")
		c.Remove("gonedir")
		r := c.Save("--session", "dd", "--profile-path", "dd.glaze")
		c.OK(r, "save of a pane whose directory was deleted")
		c.Match("save warns about the deleted directory", "no longer exists", r.Stderr)
		c.NoMatch("save leaves the deleted directory out", "gonedir", c.Read("dd.glaze"))
		c.OK(c.Glaze("format", "--validate", "--profile-path", "dd.glaze"), "the saved profile validates")
	})

	// tmux titles a new pane with the host name and names a window after its program; neither belongs in a profile.
	harness.Run(t, "save_default_names", func(c *harness.Case) {
		c.TmuxSetup("new-session", "-d", "-s", "raw")
		c.TmuxSetup("new-window", "-t", "=raw:", "-n", "two")
		automaticName := c.Tmux("display-message", "-p", "-t", "=raw:^", "#{window_name}")
		host := c.Tmux("display-message", "-p", "-t", "=raw:", "#{host}")
		r := c.Save("--session", "raw", "--stdout")
		c.OK(r, "save of a session with default names")
		c.NoMatch("save leaves out the host name", `"`+regexp.QuoteMeta(host)+`"`, r.Stdout)
		c.NoMatch("save leaves out the automatic window name", `name += "`+regexp.QuoteMeta(automaticName)+`"`, r.Stdout)
		c.Match("save keeps a window name that someone chose", `name += "two"`, r.Stdout)
	})
}
