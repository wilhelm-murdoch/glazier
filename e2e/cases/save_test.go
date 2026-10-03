package cases

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// saveRawLayout matches the layout line that save writes. The geometry of a preset differs between tmux versions.
var saveRawLayout = regexp.MustCompile(`(?m)^(\s*layout\s*=\s*)"[^"]*"$`)

func TestSave(t *testing.T) {
	harness.Run(t, "save_roundtrip", func(c *harness.Case) {
		c.Mkdir("d/a", "d/b")
		c.Fixture("save/rich.glaze")
		c.OK(c.Up(), "up rich profile")
		s1 := saveSnap(c, "sv")
		r := c.Glaze("save", "--session", "sv", "--profile-path", "saved.glaze", "--socket-name", c.Socket)
		c.OK(r, "save to file")
		c.NoMatch("save shows no EXPERIMENTAL warning", "EXPERIMENTAL", r.Stderr)
		saved := c.Read("saved.glaze")
		c.Logf("saved profile:\n%s", saved)
		c.OK(c.Glaze("format", "--validate", "--profile-path", "saved.glaze"), "saved profile validates")
		c.Down()
		c.OK(c.Up("--profile-path", "saved.glaze"), "up from saved profile")
		c.EventuallyEqual("round-trip structure, focus, paths and geometry", s1, func() string { return saveSnap(c, "sv") })
		for _, k := range []string{"commands", "envs", "hooks", "options", "hunter2", "SECRET"} {
			c.NoMatch("save excludes "+k, k, saved)
		}
	})

	harness.Run(t, "save_stdout_clean", func(c *harness.Case) {
		c.Mkdir("d/a", "d/b")
		c.Fixture("save/rich.glaze")
		c.Up()
		r := c.Glaze("save", "--session", "sv", "--stdout", "--socket-name", c.Socket)
		c.OK(r, "save --stdout")
		c.NoMatch("save --stdout has no log lines on stdout", "INF|WRN|EXPERIMENTAL", r.Stdout)
		c.Match("save --stdout emits HCL", `^session \{`, r.Stdout)
		c.Golden("save --stdout writes the declared structure", "save/rich.glaze", saveRawLayout.ReplaceAllString(r.Stdout, `${1}"RAW"`))
		c.Equal("save --stdout writes no file", ".glaze", saveGlazeFiles(c))
		c.Write("piped.glaze", strings.TrimRight(r.Stdout, "\n")+"\n")
		c.OK(c.Glaze("format", "--validate", "--profile-path", "piped.glaze"), "piped save output validates")
	})

	harness.Run(t, "save_default_overwrite", func(c *harness.Case) {
		c.Simple("ow")
		c.Write(".glaze", c.Read(".glaze")+"# my hand-written profile\n")
		c.Up()
		before := c.Read(".glaze")
		r := c.Glaze("save", "--session", "ow", "--socket-name", c.Socket)
		c.Fails(r, "save refuses an existing .glaze")
		c.Match("save refusal says to use --force", "use --force", r.Stderr)
		c.True("save does not clobber existing .glaze", before == c.Read(".glaze"), "hand-written .glaze overwritten without prompt or backup (comment lost)")
		c.OK(c.Glaze("save", "--session", "ow", "--force", "--socket-name", c.Socket), "save --force replaces .glaze")
		// The hand-written profile has no focus attribute, and save always writes one.
		c.Match("save --force wrote the session", `focus += true`, c.Read(".glaze"))
	})

	harness.Run(t, "save_force_symlink", func(c *harness.Case) {
		c.Simple("sfs", "real.glaze")
		c.Up("--profile-path", "real.glaze")
		c.Chmod("real.glaze", 0o600)
		c.Symlink("real.glaze", ".glaze")
		c.OK(c.Glaze("save", "--session", "sfs", "--force", "--socket-name", c.Socket), "save --force through a symlink")
		info, err := os.Lstat(c.Path(".glaze"))
		c.True("save keeps the symlink", err == nil && info.Mode()&os.ModeSymlink != 0, "save replaced the symlink with a regular file")
		mode := "missing"
		if info, err := os.Stat(c.Path("real.glaze")); err == nil {
			mode = fmt.Sprintf("%o", info.Mode().Perm())
		}
		c.Equal("save keeps the permissions", "600", mode)
		c.Match("save wrote through the symlink", `focus += true`, c.Read("real.glaze"))
	})

	harness.Run(t, "save_outside_tmux", func(c *harness.Case) {
		c.Simple("so")
		c.Up()
		r := c.Glaze("save", "--stdout", "--socket-name", c.Socket)
		c.Fails(r, "save without --session outside tmux")
		c.Match("save outside tmux asks for --session", "--session", r.Stderr)
		c.Equal("save outside tmux writes no profile", "", r.Stdout)
	})

	harness.Run(t, "save_missing_session", func(c *harness.Case) {
		c.Simple("sm")
		c.Up()
		c.Fails(c.Glaze("save", "--session", "nope", "--stdout", "--socket-name", c.Socket), "save of unknown session")
	})

	harness.Run(t, "save_prefix_session", func(c *harness.Case) {
		c.Tmux("new-session", "-d", "-s", "project-long")
		r := c.Glaze("save", "--session", "project", "--stdout", "--socket-name", c.Socket)
		c.True("save --session does not prefix-match", r.Code != 0 && !r.TimedOut, "saved %q", firstLine(r.Stdout))
	})

	harness.Run(t, "save_no_server", func(c *harness.Case) {
		c.Fails(c.Glaze("save", "--session", "x", "--stdout", "--socket-name", c.Socket), "save with no server")
	})

	harness.Run(t, "save_foreign_session", func(c *harness.Case) {
		c.Mkdir("fdir")
		c.Tmux("new-session", "-d", "-s", "raw", "-c", c.Path("fdir"))
		c.Tmux("split-window", "-h", "-t", "=raw:")
		c.Tmux("new-window", "-t", "=raw:", "-n", "two")
		s1 := saveSnapNoNames(c, "raw")
		c.OK(c.Glaze("save", "--session", "raw", "--profile-path", "raw.glaze", "--socket-name", c.Socket), "save non-glaze session")
		c.Logf("saved profile:\n%s", c.Read("raw.glaze"))
		c.Tmux("kill-session", "-t", "=raw")
		c.OK(c.Up("--profile-path", "raw.glaze"), "up from foreign save")
		c.EventuallyEqual("foreign round-trip (ignoring titles)", s1, func() string { return saveSnapNoNames(c, "raw") })
	})

	harness.Run(t, "save_escaping", func(c *harness.Case) {
		c.Fixture("save/escaping.glaze")
		// glaze replaces a backslash in a pane name with -, because tmux 3.7 and later store it escaped.
		c.OK(c.Up(), "up with escape-worthy names")
		c.Equal("window name literal", `q"uote`, c.WindowNames("esc"))
		c.Equal("pane names literal", "${dollar},%{pct},back-slash", c.PaneTitles("=esc:"))
		c.OK(c.Glaze("save", "--session", "esc", "--profile-path", "s.glaze", "--socket-name", c.Socket), "save escape-worthy names")
		c.Logf("saved profile:\n%s", c.Read("s.glaze"))
		c.Down()
		c.OK(c.Up("--profile-path", "s.glaze"), "up from saved escaped profile")
		c.Equal("escaped window name round-trips", `q"uote`, c.WindowNames("esc"))
		c.Equal("escaped pane names round-trip", "${dollar},%{pct},back-slash", c.PaneTitles("=esc:"))
	})

	// glaze does not keep a pane title that a program in the pane changes.
	harness.Run(t, "save_titles_changed", func(c *harness.Case) {
		c.Fixture("save/title-command.glaze")
		c.Up()
		c.EventuallyEqual("pane title after app sets it", "changed-by-app", func() string { return c.PaneTitles("=tc:w") })
	})

	harness.Run(t, "save_deleted_dir", func(c *harness.Case) {
		c.Mkdir("gonedir")
		c.Tmux("new-session", "-d", "-s", "dd", "-c", c.Path("gonedir"))
		c.Remove("gonedir")
		r := c.Glaze("save", "--session", "dd", "--profile-path", "dd.glaze", "--socket-name", c.Socket)
		c.OK(r, "save of a pane whose directory was deleted")
		c.Match("save warns about the deleted directory", "no longer exists", r.Stderr)
		c.NoMatch("save leaves the deleted directory out", "gonedir", c.Read("dd.glaze"))
		c.OK(c.Glaze("format", "--validate", "--profile-path", "dd.glaze"), "the saved profile validates")
	})

	// tmux titles a new pane with the host name and names a window after its program; neither belongs in a profile.
	harness.Run(t, "save_default_names", func(c *harness.Case) {
		c.Tmux("new-session", "-d", "-s", "raw")
		c.Tmux("new-window", "-t", "=raw:", "-n", "two")
		auto := c.Tmux("display-message", "-p", "-t", "=raw:^", "#{window_name}")
		host := c.Tmux("display-message", "-p", "-t", "=raw:", "#{host}")
		r := c.Glaze("save", "--session", "raw", "--stdout", "--socket-name", c.Socket)
		c.OK(r, "save of a session with default names")
		c.NoMatch("save leaves out the host name", `"`+regexp.QuoteMeta(host)+`"`, r.Stdout)
		c.NoMatch("save leaves out the automatic window name", `name += "`+regexp.QuoteMeta(auto)+`"`, r.Stdout)
		c.Match("save keeps a window name that someone chose", `name += "two"`, r.Stdout)
	})

	harness.Run(t, "up_renamed_window", func(c *harness.Case) {
		c.TmuxConf("set-hook -g after-new-window 'rename-window renamed'\n")
		c.Fixture("save/two-windows.glaze")
		r := c.Up()
		c.OK(r, "up with a tmux.conf hook that renames windows")
		c.Match("up warns that tmux renamed a window", `tmux renamed the window.*window=second.*name=renamed`, r.Stderr)
	})
}

// saveSnap returns every window and pane of a session with name, focus, path and geometry.
func saveSnap(c *harness.Case, session string) string {
	var lines []string
	for _, w := range strings.Split(c.Tmux("list-windows", "-t", "="+session, "-F", "#{window_id}"), "\n") {
		lines = append(lines, c.Tmux("display-message", "-p", "-t", w, "W #{window_index}|#{window_name}|#{window_active}|#{window_panes}"))
		lines = append(lines, strings.Split(c.Tmux("list-panes", "-t", w, "-F", "  P #{pane_index}|#{pane_title}|#{pane_active}|#{pane_current_path}|#{pane_left},#{pane_top},#{pane_width}x#{pane_height}"), "\n")...)
	}
	return strings.Join(lines, "\n")
}

// saveSnapNoNames is saveSnap without window names, pane titles and window indexes.
func saveSnapNoNames(c *harness.Case, session string) string {
	lines := strings.Split(saveSnap(c, session), "\n")
	for i, line := range lines {
		fields := strings.Split(line, "|")
		if len(fields) < 2 {
			continue
		}
		head := fields[0]
		if strings.HasPrefix(head, "W ") {
			head = "W"
		}
		lines[i] = strings.Join(append([]string{head}, fields[2:]...), "|")
	}
	return strings.Join(lines, "\n")
}

// saveGlazeFiles returns the names in the work directory that contain "glaze", joined with commas.
func saveGlazeFiles(c *harness.Case) string {
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		c.T().Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.Contains(e.Name(), "glaze") {
			names = append(names, e.Name())
		}
	}
	return strings.Join(names, ",")
}
