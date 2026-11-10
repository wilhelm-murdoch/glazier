package cases

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// The fixed lines of a diff report, as patterns on one line of stdout.
const (
	diffNoChanges         = `(?m)^No changes\. Session %s matches the profile in everything glaze can compare\.$`
	diffNotComparedState  = `(?m)^Not compared: commands, envs, hooks and options\. tmux does not report what glaze set\.$`
	diffNotComparedLayout = `(?m)^Not compared: size, adjust and layout\. tmux reports them in a form that changes with the terminal size\.$`
	diffHintApply         = `(?m)^Run glaze up --clear with the same profile and flags to apply the profile\. The kill ends every process in the session\.$`
	diffHintApplyInside   = `(?m)^Run glaze up --clear with the same profile and flags from another session or outside tmux\. The kill ends every process in the session, including this shell\.$`
	diffHintCreate        = `(?m)^Run glaze up with the same profile and flags to create the session\.$`

	// diffIdiom is the documented shell line that applies the profile only on exit 5; %[1]s is glaze and %[2]s the socket.
	diffIdiom = `%[1]s diff -q --socket-name %[2]s; [ $? -eq 5 ] && %[1]s up --detached --clear --socket-name %[2]s`

	// diffHostileProfile has a window name with a quote and pane names that tmux and a shell treat as special.
	diffHostileProfile = `session {
  name = "hostile-diff"
  window {
    name = "q\"uote"
    pane {
      name = "#{pane_id}"
    }
    pane {
      name = "--"
    }
  }
  window {
    name = "-t x"
    pane {
      name = "p;"
    }
  }
}
`

	// diffHostileLines is the number of lines of the creation plan of diffHostileProfile: six items, the Plan line, two Not compared lines and the hint.
	diffHostileLines = 10

	// diffNamelessProfile is a registry profile without a session name, which diff must refuse.
	diffNamelessProfile = "session {\n  window {\n    pane {}\n  }\n}\n"
)

// diff compares a profile with the session that runs under its name and prints what up --clear would change.
func TestDiff(t *testing.T) {
	// up then diff is empty on every target: the invariant next to up_twice.
	harness.Run(t, "diff_no_changes", func(c *harness.Case) {
		c.Mkdir("d/a", "d/b")
		c.Fixture("save/rich.glaze")
		c.Must(c.Up(), "up")
		c.WaitForPanes("save-rich")
		before := c.State("save-rich", harness.WindowState, harness.PaneState)
		r := c.Diff()
		c.OK(r, "diff of an unchanged session")
		c.Match("diff of an unchanged session says no changes", fmt.Sprintf(diffNoChanges, `"save-rich"`), r.Stdout)
		c.Match("diff of an unchanged session names the state it does not compare", diffNotComparedState, r.Stdout)
		c.Match("diff of an unchanged session names the geometry it does not compare", diffNotComparedLayout, r.Stdout)
		c.NoMatch("diff of an unchanged session prints no hint", `Run glaze`, r.Stdout)
		c.Golden("diff of an unchanged session output", "diff/no-changes.txt", r.Stdout)
		c.NoMatch("diff has no log lines on stdout", "INF|WRN|ERR", r.Stdout)
		c.Equal("diff of an unchanged session writes nothing to stderr", "", r.Stderr)
		if !c.Equal("diff leaves the session as it was", before, c.State("save-rich", harness.WindowState, harness.PaneState)) {
			c.Snapshot("save-rich")
		}

		c.Equal("diff writes no file", ".glaze", workFiles(c, "*glaze*"))
	})

	// The user renamed misc, opened extra, killed solo, split edit, retitled tests, replaced the panes of run and moved the focus and a directory.
	harness.Run(t, "diff_every_kind", func(c *harness.Case) {
		c.Mkdir("d/b", "d/logs")
		c.Fixture("diff/every-kind.glaze")
		c.Must(c.Up(), "up")
		c.WaitForPanes("every-kind")
		c.TmuxSetup("rename-window", "-t", "=every-kind:misc", "notes")
		c.TmuxSetup("new-window", "-t", "=every-kind:", "-n", "extra")
		c.TmuxSetup("kill-window", "-t", "=every-kind:solo")
		c.TmuxSetup("split-window", "-t", "=every-kind:edit", "-c", c.Path("d"))
		c.TmuxSetup("select-pane", "-t", paneID(c, "=every-kind:edit", "tests"), "-T", "psql")
		c.TmuxSetup("split-window", "-t", "=every-kind:run", "-c", c.Path("d"))
		c.TmuxSetup("kill-pane", "-t", paneID(c, "=every-kind:run", "server"))
		c.TmuxSetup("kill-pane", "-t", paneID(c, "=every-kind:run", "logs"))
		c.TmuxSetup("select-window", "-t", "=every-kind:run")
		c.Type(paneID(c, "=every-kind:edit", "shell"), "cd "+harness.ShellQuote(c.Path("d/b")))
		c.EventuallyPanePaths("the shell pane moved to another directory", c.Path("d/b")+","+c.Path("d")+","+c.Path("d"), "=every-kind:edit")
		c.WaitForPanes("every-kind")
		changed := c.State("every-kind", harness.WindowState, harness.PaneState)
		r := c.Diff()
		c.ExitCode(r, "diff of a changed session exits 5", exitDifferences)
		c.Match("diff reports the renamed window as a removal", `- window "notes" \[3\]`, r.Stdout)
		c.Match("diff reports the renamed window as an addition", `\+ window "misc" \[4\]`, r.Stdout)
		c.Match("diff reports the extra window", `- window "extra" \[4\]`, r.Stdout)
		c.Match("diff reports the killed window", `\+ window "solo" \[3\]`, r.Stdout)
		c.Match("diff reports the killed pane with its directory", `\+ pane "logs" \[2\]  `+regexp.QuoteMeta(c.Path("d/logs")), r.Stdout)
		c.Match("diff shows the extra pane as unnamed", `- pane \(unnamed\) \[3\]`, r.Stdout)
		c.Match("diff counts a replaced pane as a change", `~ title: \(unnamed\) -> "server"; glaze titles every pane it creates`, r.Stdout)
		c.Match("diff shows the retitled pane as to check", `(?m)^\s+\? title: "psql", profile name "tests"; a program in the pane can set the title$`, r.Stdout)
		c.Match("diff shows the moved directory as to check", `(?m)^\s+\? directory: `+quoted(c.Path("d/b"))+`, profile `+quoted(c.Path("d"))+`; tmux reports where the shell is now`, r.Stdout)
		c.Match("diff shows the moved window focus as to check", `\? focus: window "run" \[2\], profile window "edit" \[1\]; a user can select another window`, r.Stdout)
		c.Match("diff shows the moved pane focus as to check", `\? focus: pane \(unnamed\) \[3\], profile pane "tests" \[2\]; a user can select another pane`, r.Stdout)
		c.Match("diff counts the plan", `(?m)^Plan: 5 to add, 1 to change, 5 to remove, 4 to check\.$`, r.Stdout)
		c.Match("diff names the apply step", diffHintApply, r.Stdout)
		c.Golden("diff of a changed session output", "diff/every-kind.txt", r.Stdout)
		c.Equal("diff of a changed session writes nothing to stderr", "", r.Stderr)
		c.NoMatch("diff prints no escape codes", `\x1b\[`, r.Output())
		c.Equal("diff output is the same with NO_COLOR", r.Stdout, c.GlazeWith(harness.Opts{Env: []string{"NO_COLOR=1"}}, "diff", "--socket-name", c.Socket).Stdout)
		if !c.Equal("diff changes nothing", changed, c.State("every-kind", harness.WindowState, harness.PaneState)) {
			c.Snapshot("every-kind")
		}
	})

	harness.Run(t, "diff_not_running", func(c *harness.Case) {
		c.Mkdir("d/logs")
		c.Fixture("diff/every-kind.glaze")
		r := c.Diff()
		c.ExitCode(r, "diff of a session that does not run exits 5", exitDifferences)
		c.Golden("diff with no server prints a creation plan", "diff/not-running.txt", r.Stdout)
		c.Match("diff with no server names the create step", diffHintCreate, r.Stdout)
		c.NoMatch("diff with no server does not name --clear", `--clear`, r.Stdout)
		c.Equal("diff with no server writes nothing to stderr", "", r.Stderr)
		c.NoServer("diff with no server")
		c.TmuxSetup("new-session", "-d", "-s", "bystander")
		r = c.Diff()
		c.ExitCode(r, "diff on a server without the session exits 5", exitDifferences)
		c.Match("diff on a server without the session prints a creation plan", `(?m)^\+ session "every-kind"  `+regexp.QuoteMeta(c.Path("d"))+`$`, r.Stdout)
		c.SessionExists("diff leaves other sessions alone", "bystander")
		c.SessionGone("diff does not create the session", "every-kind")
		c.Simple("nodir", "nodir.glaze")
		r = c.Diff("--profile-path", "nodir.glaze")
		c.ExitCode(r, "diff of a session without a directory that does not run exits 5", exitDifferences)
		c.Golden("a creation plan marks an undeclared directory", "diff/not-running-nodir.txt", r.Stdout)
		c.NoMatch("a creation plan without a directory prints no path", regexp.QuoteMeta(c.Dir), r.Stdout)
	})

	harness.Run(t, "diff_quiet", func(c *harness.Case) {
		c.Mkdir("d/a", "d/b")
		c.Fixture("save/rich.glaze")
		c.Must(c.Up(), "up")
		c.WaitForPanes("save-rich")
		r := c.Diff("-q")
		c.OK(r, "quiet diff of an unchanged session")
		c.Equal("quiet diff of an unchanged session prints nothing on stdout", "", r.Stdout)
		before := c.State("save-rich", harness.WindowState, harness.PaneState)
		idiom := fmt.Sprintf(diffIdiom, harness.ShellQuote(c.Env().Glaze), harness.ShellQuote(c.Socket))
		c.Exec(harness.Opts{}, "sh", "-c", idiom)
		c.Equal("the exit 5 idiom leaves a matching session", before, c.State("save-rich", harness.WindowState, harness.PaneState))
		c.TmuxSetup("new-window", "-t", "=save-rich:", "-n", "extra")
		r = c.Diff("--quiet")
		c.ExitCode(r, "quiet diff exits 5 when the session differs", exitDifferences)
		c.Equal("quiet diff prints nothing on stdout", "", r.Stdout)
		c.Equal("quiet diff prints nothing on stderr", "", r.Stderr)
		c.SessionExists("quiet diff leaves the session", "save-rich")
		c.OK(c.Exec(harness.Opts{}, "sh", "-c", idiom), "the exit 5 idiom")
		c.Equal("the exit 5 idiom runs up --clear", "edit,run,solo", c.WindowNames("save-rich"))
	})

	// Commands, envs, hooks, options, layouts and sizes are not compared, so another profile with the same structure matches.
	harness.Run(t, "diff_not_compared", func(c *harness.Case) {
		c.Mkdir("d/a", "d/b")
		c.Fixture("save/rich.glaze")
		c.Must(c.Up(), "up")
		c.WaitForPanes("save-rich")
		before := c.State("save-rich", harness.WindowState, harness.PaneState)
		c.Fixture("diff/rich-other-settings.glaze")
		r := c.Diff()
		c.OK(r, "changed commands, envs, hooks and options are no difference")
		c.Match("diff with changed settings says no changes", fmt.Sprintf(diffNoChanges, `"save-rich"`), r.Stdout)
		c.Match("diff with changed settings names what it does not compare", diffNotComparedState, r.Stdout)
		c.NoMatch("diff does not print the secret", "hunter2", r.Output())
		c.NoMatch("diff does not print the command text", "echo", r.Output())
		c.Equal("diff with changed settings leaves the session", before, c.State("save-rich", harness.WindowState, harness.PaneState))
	})

	// rawLayout stops its server, so it runs first.
	harness.Run(t, "diff_layout_not_compared", func(c *harness.Case) {
		raw := rawLayout(c, 3)
		c.Fixture("layouts/three-panes.glaze")
		c.Must(c.Up("--var", "layout=main-vertical"), "up with a preset")
		c.WaitForPanes("three-panes")
		c.TmuxSetup("select-layout", "-t", "=three-panes:w", "even-horizontal")
		c.TmuxSetup("resize-pane", "-t", paneID(c, "=three-panes:w", "p1"), "-L", "5")
		changed := c.Geometry("=three-panes:w")
		r := c.Diff("--var", "layout=main-vertical")
		c.OK(r, "a changed layout is no difference")
		c.Match("diff after a layout change says no changes", fmt.Sprintf(diffNoChanges, `"three-panes"`), r.Stdout)
		c.Match("diff names layout as not compared", diffNotComparedLayout, r.Stdout)
		c.Equal("diff after a layout change leaves the geometry", changed, c.Geometry("=three-panes:w"))
		c.KillServer()
		c.Must(c.Up("--var", "layout="+raw), "up with a raw layout")
		c.WaitForPanes("three-panes")
		r = c.Diff("--var", "layout="+raw)
		c.OK(r, "a replayed raw layout shows no difference")
		c.NoMatch("diff prints no layout line", `(?m)^\s+.*layout`, r.Stdout)
		c.TmuxSetup("select-layout", "-t", "=three-panes:w", "tiled")
		c.OK(c.Diff("--var", "layout="+raw), "a changed raw layout is no difference")
	})

	harness.Run(t, "diff_names", func(c *harness.Case) {
		c.Fixture("diff/odd-names.glaze")
		c.Must(c.Up(), "up with odd names")
		c.WaitForPanes("a-b-c")
		r := c.Diff()
		c.OK(r, "diff of sanitized names")
		c.Match("diff warns about the sanitized session name", `replacing them with hyphens.*tmux_name=a-b-c`, r.Stderr)
		c.Match("diff compares the session under its tmux name", fmt.Sprintf(diffNoChanges, `"a-b-c"`), r.Stdout)
		c.Mkdir("d")
		c.Simple("foreign", "foreign.glaze")
		c.TmuxSetup("new-session", "-d", "-s", "foreign", "-c", c.Path("d"))
		c.WaitForPanes("foreign")
		host := c.Tmux("display-message", "-p", "-t", "=foreign:", "#{host}")
		r = c.Diff("--profile-path", "foreign.glaze")
		c.ExitCode(r, "diff of a foreign session exits 5", exitDifferences)
		c.Match("diff shows a tmux-named window as removed with a note", `(?m)^  - window ".*" \[1\] \(named by tmux\)$`, r.Stdout)
		c.Match("diff shows the default pane title as unnamed", `(?m)^    - pane \(unnamed\) \[1\]$`, r.Stdout)
		c.Match("diff adds the declared window", `(?m)^  \+ window "w" \[1\]$`, r.Stdout)
		c.Match("diff adds the declared pane with the current directory token", `(?m)^    \+ pane "p" \[1\]  \(current directory\)$`, r.Stdout)
		c.NoMatch("diff never prints the host name", `"`+regexp.QuoteMeta(host)+`"`, r.Output())
		c.Must(c.Up("--profile-path", "foreign.glaze", "--clear"), "up --clear over the foreign session")
		c.WaitForPanes("foreign")
		r = c.Diff("--profile-path", "foreign.glaze")
		c.OK(r, "diff after up --clear")
		c.Match("diff after up --clear says no changes", fmt.Sprintf(diffNoChanges, `"foreign"`), r.Stdout)
		c.Write("hostile.glaze", diffHostileProfile)
		r = c.Diff("--profile-path", "hostile.glaze")
		c.ExitCode(r, "diff of hostile names that do not run exits 5", exitDifferences)
		c.Equal("diff of hostile names renders one line per item", strconv.Itoa(diffHostileLines), strconv.Itoa(strings.Count(r.Stdout, "\n")))
		c.Match("diff quotes a quote in a name", `(?m)^  \+ window "q\\"uote" \[1\]$`, r.Stdout)
		c.Match("diff quotes a format in a name", `(?m)^    \+ pane "#\{pane_id\}" \[1\]  \(current directory\)$`, r.Stdout)
		c.Must(c.Up("--profile-path", "hostile.glaze"), "up with hostile names")
		c.WaitForPanes("hostile-diff")
		r = c.Diff("--profile-path", "hostile.glaze")
		c.OK(r, "diff reads hostile names back")
		c.Match("diff of hostile names says no changes", fmt.Sprintf(diffNoChanges, `"hostile-diff"`), r.Stdout)
	})

	// A window whose profile turns automatic-rename on is matched by that option, because tmux names it after its program.
	harness.Run(t, "diff_auto_rename", func(c *harness.Case) {
		c.Fixture("diff/auto-rename.glaze")
		c.Must(c.Up(), "up")
		c.WaitForPanes("auto-rename")
		c.Equal("up turns automatic-rename on", "on", c.Option("=auto-rename:^", "automatic-rename", "-w"))
		r := c.Diff()
		c.OK(r, "a window the profile lets tmux name is no difference")
		c.Match("diff with a tmux-named profile window says no changes", fmt.Sprintf(diffNoChanges, `"auto-rename"`), r.Stdout)
		c.NoMatch("diff with a tmux-named profile window prints no window line", `window`, r.Stdout)
		c.Equal("diff with a tmux-named profile window writes nothing to stderr", "", r.Stderr)
		c.TmuxSetup("rename-window", "-t", "=auto-rename:^", "byhand")
		r = c.Diff()
		c.ExitCode(r, "a renamed tmux-named window is a difference", exitDifferences)
		c.Match("a renamed tmux-named window is an addition", `(?m)^  \+ window "edit" \[1\]$`, r.Stdout)
		c.Match("a renamed tmux-named window is a removal without a note", `(?m)^  - window "byhand" \[1\]$`, r.Stdout)
		c.Match("a renamed tmux-named window counts in the plan", `(?m)^Plan: 2 to add, 0 to change, 2 to remove\.$`, r.Stdout)
	})

	harness.Run(t, "diff_directories", func(c *harness.Case) {
		c.Mkdir("d/a", "d/b", "d/x", "other")
		c.Fixture("diff/session-dir.glaze")
		c.Must(c.Up("--var", "dir=d"), "up")
		c.WaitForPanes("session-dir")
		c.Type(paneID(c, "=session-dir:w", "shell"), "cd "+harness.ShellQuote(c.Path("d/b")))
		c.EventuallyPanePaths("the shell pane moved", c.Path("d/b")+","+c.Path("d/a"), "=session-dir:w")
		r := c.Diff("--var", "dir=d")
		c.OK(r, "a cd in a pane is not a counted difference")
		c.Match("a cd in a pane is shown as to check", `(?m)^      \? directory: `+quoted(c.Path("d/b"))+`, profile `+quoted(c.Path("d"))+`; tmux reports where the shell is now, not where the pane started$`, r.Stdout)
		c.Match("a cd in a pane counts as to check", `(?m)^Plan: 0 to add, 0 to change, 0 to remove, 1 to check\.$`, r.Stdout)
		c.NoMatch("a diff with only checks prints no hint", `Run glaze`, r.Stdout)
		r = c.Diff("--var", "dir=other")
		c.ExitCode(r, "a changed session directory exits 5", exitDifferences)
		c.Match("diff reports the session directory", `(?m)^  ~ starting_directory: `+quoted(c.Path("d"))+` -> `+quoted(c.Path("other"))+`$`, r.Stdout)
		c.Match("diff reports a pane that inherits the changed directory as to check", `(?m)^      \? directory: `+quoted(c.Path("d/b"))+`, profile `+quoted(c.Path("other"))+`;`, r.Stdout)
		c.Must(c.Down("--var", "dir=d"), "down")
		c.Must(c.Up("--var", "dir=d/x"), "up in a directory that the case removes")
		c.WaitForPanes("session-dir")
		c.Remove("d/x")
		r = c.Diff("--var", "dir=d/b")
		c.ExitCode(r, "a deleted session directory exits 5", exitDifferences)
		c.Match("diff marks a live directory that no longer exists", `(?m)^  ~ starting_directory: `+quoted(c.Path("d/x"))+` \(no longer exists\) -> `+quoted(c.Path("d/b"))+`$`, r.Stdout)
		c.Simple("nodir", "nodir.glaze")
		c.Cd("d")
		c.Must(c.Up("--profile-path", c.Path("nodir.glaze")), "up without a session directory")
		c.WaitForPanes("nodir")
		c.Cd("other")
		r = c.Diff("--profile-path", c.Path("nodir.glaze"))
		c.OK(r, "an undeclared session directory is not a counted difference")
		c.Match("an undeclared session directory is to check", `(?m)^  \? starting_directory: `+quoted(c.Path("d"))+`, profile declares none; up would use the current directory `+quoted(c.Path("other"))+`$`, r.Stdout)
		c.NoMatch("a pane that inherits an undeclared directory is compared with the session path", `\? directory:`, r.Stdout)
	})

	harness.Run(t, "diff_registry_name", func(c *harness.Case) {
		c.Simple("reg-diff", registryHome+"/work.glaze")
		c.Mkdir("empty", "proj")
		c.Cd("empty")
		c.Must(c.Up("work"), "up by name")
		c.WaitForPanes("reg-diff")
		before := c.State("reg-diff", harness.WindowState, harness.PaneState)
		r := c.Diff("work")
		c.OK(r, "diff by name")
		c.Match("diff by name says no changes", fmt.Sprintf(diffNoChanges, `"reg-diff"`), r.Stdout)
		c.Equal("diff by name leaves the session", before, c.State("reg-diff", harness.WindowState, harness.PaneState))
		c.ExitCode(c.Diff("work", "--profile-path", "x.glaze"), "diff with a name and --profile-path is a usage error (exit 2)", exitUsage)
		c.ExitCode(c.Diff("work", "api"), "diff with a second word is a usage error (exit 2)", exitUsage)
		r = c.Diff("nope")
		c.ExitCode(r, "diff of an unknown name exits 3", exitInvalidProfile)
		c.Match("diff of an unknown name points at ls --profiles", "ls --profiles", r.Stderr)
		c.Equal("diff of an unknown name prints nothing on stdout", "", r.Stdout)
		c.Write(registryHome+"/noname.glaze", diffNamelessProfile)
		r = c.Diff("noname")
		c.ExitCode(r, "diff of a nameless registry profile exits 3", exitInvalidProfile)
		c.Match("diff of a nameless registry profile names the cause", "Missing session name", r.Stderr)
		c.Fixture("registry/per-project.glaze", registryHome+"/per.glaze")
		c.Cd("proj")
		c.Must(c.Up("per"), "up with a path.base name")
		c.WaitForPanes("reg-proj")
		r = c.Diff("per")
		c.OK(r, "diff of a path.base session from its directory")
		c.Cd("empty")
		r = c.Diff("per")
		c.ExitCode(r, "diff of a path.base session from another directory exits 5", exitDifferences)
		c.Match("diff of a path.base session from another directory names the cause", "depends on the current directory", r.Stderr)
		c.Match("diff of a path.base session from another directory plans the other name", `(?m)^\+ session "reg-empty"  \(current directory\)$`, r.Stdout)
		r = c.Glaze("diff", "--generate-shell-completion")
		c.Match("diff candidates include work", "(?m)^work$", r.Stdout)
		c.Equal("diff candidates write nothing to stderr", "", r.Stderr)
	})

	harness.Run(t, "diff_invalid_profile", func(c *harness.Case) {
		c.Fixture("malformed/unknown-attribute.glaze")
		r := c.Diff()
		c.ExitCode(r, "diff of an invalid profile exits 3", exitInvalidProfile)
		c.Equal("diff of an invalid profile prints nothing on stdout", "", r.Stdout)
		c.Match("diff of an invalid profile shows the diagnostic", "Unsupported argument", r.Stderr)
		c.Fixture("variables/typed.glaze", "typed.glaze")
		r = c.Diff("--profile-path", "typed.glaze")
		c.ExitCode(r, "diff with a missing variable exits 3", exitInvalidProfile)
		c.Match("diff with a missing variable names the variable", `Required variable not set(?s).*"fixer"`, r.Stderr)
		c.Write("vars.hcl", "fixer = \"file\"\n")
		r = c.Diff("--profile-path", "typed.glaze", "--var-file", "vars.hcl", "--var", "count=3")
		c.ExitCode(r, "diff with --var-file and --var plans the session", exitDifferences)
		c.Match("diff resolves the variables of the plan", `(?m)^  \+ window "file-3-false" \[1\]$`, r.Stdout)
		c.NoServer("diff of an invalid profile")
	})

	// A moved focus is a question, not a counted difference, because a user moves it with one key.
	harness.Run(t, "diff_focus", func(c *harness.Case) {
		c.Fixture("focus/second-window.glaze")
		c.Must(c.Up(), "up")
		c.WaitForPanes("second-window")
		c.TmuxSetup("select-pane", "-t", paneID(c, "=second-window:w1", "c"))
		c.TmuxSetup("select-window", "-t", "=second-window:w2")
		before := c.State("second-window", harness.WindowState, harness.PaneState)
		r := c.Diff()
		c.OK(r, "a moved focus is not a counted difference")
		c.Match("a moved window focus is to check", `(?m)^  \? focus: window "w2" \[2\], profile window "w1" \[1\]; a user can select another window$`, r.Stdout)
		c.Match("a moved pane focus is to check", `(?m)^    \? focus: pane "c" \[3\], profile pane "b" \[2\]; a user can select another pane$`, r.Stdout)
		c.Match("a moved focus counts as to check", `(?m)^Plan: 0 to add, 0 to change, 0 to remove, 2 to check\.$`, r.Stdout)
		c.NoMatch("a diff with only a moved focus prints no hint", `Run glaze`, r.Stdout)
		c.Golden("diff of a moved focus output", "diff/focus.txt", r.Stdout)
		c.Equal("diff of a moved focus leaves the focus", before, c.State("second-window", harness.WindowState, harness.PaneState))
	})

	harness.Run(t, "diff_order", func(c *harness.Case) {
		c.Fixture("diff/order.glaze")
		c.Must(c.Up(), "up")
		c.WaitForPanes("order")
		c.TmuxSetup("swap-window", "-s", "=order:a", "-t", "=order:c")
		r := c.Diff()
		c.ExitCode(r, "a changed window order exits 5", exitDifferences)
		c.Match("diff reports the window order", `(?m)^  ~ order: c, b, a -> a, b, c$`, r.Stdout)
		c.Match("a changed window order counts as one change", `(?m)^Plan: 0 to add, 1 to change, 0 to remove\.$`, r.Stdout)
		c.Match("a changed window order names the apply step", diffHintApply, r.Stdout)
		c.Equal("diff of a changed window order leaves the order", "c,b,a", c.WindowNames("order"))
	})

	// glaze in a pane of the compared session gets the hint that up --clear would end that shell.
	harness.Run(t, "diff_inside_session", func(c *harness.Case) {
		c.Fixture("diff/order.glaze")
		c.Must(c.Up(), "up")
		c.WaitForPanes("order")
		c.TmuxSetup("new-window", "-t", "=order:", "-n", "extra")
		r := c.Diff()
		c.Match("diff outside the session names the apply step", diffHintApply, r.Stdout)
		out, rc := attachRun(c, c.Socket, "order", "", "inside.txt", "diff", "--socket-name", c.Socket, "--profile-path", c.Path(".glaze"))
		c.Equal("diff inside the compared session exits 5", strconv.Itoa(exitDifferences), rc)
		c.Match("diff inside the compared session says to run up --clear from outside", diffHintApplyInside, out)
		c.Match("diff inside the compared session reports the extra window", `(?m)^  - window "extra" \[4\]$`, out)
		c.SessionExists("diff inside the compared session leaves it", "order")
	})
}

// paneID returns the id (%N) of the pane with this title in a window target.
func paneID(c *harness.Case, target, title string) string {
	c.T().Helper()
	for _, line := range strings.Split(c.Tmux("list-panes", "-t", target, "-F", "#{pane_id} #{pane_title}"), "\n") {
		if id, t, _ := strings.Cut(line, " "); t == title {
			return id
		}
	}

	c.T().Fatalf("no pane %q in %s", title, target)
	return ""
}

// quoted returns a pattern that matches a path as diff prints it, through strconv.Quote.
func quoted(path string) string {
	return regexp.QuoteMeta(strconv.Quote(path))
}
