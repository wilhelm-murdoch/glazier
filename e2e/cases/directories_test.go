package cases

import (
	"runtime"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// TestDirectories checks starting_directory on each level and the directory that a pane gets without it.
func TestDirectories(t *testing.T) {
	harness.Run(t, "dirs_levels", func(c *harness.Case) {
		c.Mkdir("d/s", "d/w", "d/p", "d/with space")
		c.Fixture("directories/levels.glaze")
		c.OK(c.Up(), "up")
		c.Equal("session path", c.Path("d/s"), c.Tmux("display-message", "-p", "-t", "=levels:", "#{session_path}"))
		c.EventuallyPanePaths("pane inherits session dir", c.Path("d/s"), c.WindowID("levels", "w1"))
		c.EventuallyPanePaths("pane inherits window starting_directory", c.Path("d/w")+","+c.Path("d/p"), c.WindowID("levels", "w2"))
		c.EventuallyPanePaths("window dir with a space inherited", c.Path("d/with space"), c.WindowID("levels", "w3"))

		// An attached client opens a new window in the session path; a command-line client would use its own directory.
		cc := c.AttachControl("levels")
		cc.Send("new-window -n later")
		// Wait for the window before Close, because Close detaches the client.
		c.WaitUntil(harness.Patience, func() bool { return c.WindowID("levels", "later") != "" })
		cc.Close()
		c.EventuallyPanePaths("new window opened later by user uses session dir", c.Path("d/s"), c.WindowID("levels", "later"))
	})

	harness.Run(t, "dirs_default_cwd", func(c *harness.Case) {
		c.Simple("default-cwd", "here/.glaze")
		c.Cd("here")
		c.OK(c.Up(), "up")
		c.EventuallyPanePaths("no starting_directory uses cwd", c.Path("here"), "=default-cwd:")
	})

	harness.Run(t, "dirs_tilde", func(c *harness.Case) {
		c.Mkdir("home/proj")
		c.Fixture("directories/tilde.glaze")
		c.OK(c.Up(), "up with ~ in starting_directory")
		c.EventuallyPanePaths("~ expanded", c.Path("home/proj"), c.WindowID("tilde", "proj"))
		c.EventuallyPanePaths("bare ~ expanded", c.Home, c.WindowID("tilde", "home"))
	})

	harness.Run(t, "dirs_tilde_user", func(c *harness.Case) {
		c.Fixture("directories/tilde-user.glaze")
		r := c.Up()
		c.Fails(r, "~user rejected")
		c.NoServer("~user")
		c.Match("~user diagnostic says glaze expands only ~ and ~/", "not `~user`", r.Stderr)
	})

	harness.Run(t, "dirs_relative", func(c *harness.Case) {
		c.Mkdir("prof/sub", "elsewhere")
		c.Fixture("directories/relative.glaze", "prof/.glaze")
		c.Cd("elsewhere")
		c.OK(c.Up("--profile-path", "../prof/.glaze"), "up with a relative starting_directory from another directory")
		c.EventuallyPanePaths("relative starting_directory is relative to the profile", c.Path("prof/sub"), "=relative:")
	})

	// From a directory that was deleted, glaze reads the current directory only where the profile needs it.
	harness.Run(t, "dirs_deleted_cwd", func(c *harness.Case) {
		if runtime.GOOS == "darwin" {
			c.T().Skip("macOS still reports the path of a deleted working directory")
		}

		c.Fixture("directories/deleted-with-directory.glaze", "prof/with.glaze")
		c.Fixture("directories/deleted-without-directory.glaze", "prof/without.glaze")
		c.Fixture("directories/deleted-path-pwd.glaze", "prof/pwd.glaze")
		sessionDir, noSessionDir, usesPwd := c.Path("prof/with.glaze"), c.Path("prof/without.glaze"), c.Path("prof/pwd.glaze")

		// fromDeleted runs glaze in a directory that the shell removes just before the exec.
		fromDeleted := func(args ...string) *harness.Result {
			c.Mkdir("gone")
			return c.Exec(harness.Opts{Dir: "gone"}, "sh", append([]string{"-c", `rmdir "$PWD" && exec "$0" "$@"`, c.Env().Glaze}, args...)...)
		}

		up := []string{"up", "--detached", "--socket-name", c.Socket}

		c.OK(fromDeleted("format", "--validate", "--profile-path", sessionDir), "format --validate from a deleted directory")
		c.OK(fromDeleted(append(up, "--profile-path", sessionDir)...), "up from a deleted directory")
		c.SessionExists("session from a deleted directory", "deleted-with-directory")
		c.OK(fromDeleted("down", "--socket-name", c.Socket, "--profile-path", sessionDir), "down from a deleted directory")
		c.SessionGone("down from a deleted directory kills the session", "deleted-with-directory")

		r := fromDeleted(append(up, "--profile-path", noSessionDir)...)
		c.Fails(r, "up without a session directory from a deleted directory")
		c.Match("the error says the session needs a directory", "no starting_directory", r.Stderr)

		r = fromDeleted("format", "--validate", "--profile-path", usesPwd)
		c.Fails(r, "path.pwd from a deleted directory")
		c.Match("the error names the current directory", "Current directory not available", r.Stderr)
	})

	harness.Run(t, "dirs_missing", func(c *harness.Case) {
		c.Fixture("directories/missing.glaze")
		r := c.Up()
		c.Fails(r, "missing starting_directory rejected")
		c.NoServer("missing dir")
		c.Match("missing dir diagnostic", `[Dd]irector`, r.Stderr)
	})

	harness.Run(t, "dirs_is_file", func(c *harness.Case) {
		c.Write("afile", "")
		c.Fixture("directories/is-file.glaze")
		c.Fails(c.Up(), "starting_directory pointing at a file rejected")
		c.NoServer("starting_directory pointing at a file")
	})
}
