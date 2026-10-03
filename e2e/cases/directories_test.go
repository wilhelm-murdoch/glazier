package cases

import (
	"runtime"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

func TestDirectories(t *testing.T) {
	harness.Run(t, "dirs_levels", func(c *harness.Case) {
		c.Mkdir("d/s", "d/w", "d/p", "d/with space")
		c.Fixture("directories/levels.glaze")
		c.OK(c.Up(), "up")
		c.Equal("session path", c.Path("d/s"), c.Tmux("display-message", "-p", "-t", "=dl:", "#{session_path}"))
		c.EventuallyEqual("pane inherits session dir", c.Path("d/s"), func() string { return c.PanePaths(c.WindowID("dl", "w1")) })
		c.EventuallyEqual("pane inherits window starting_directory", c.Path("d/w")+","+c.Path("d/p"), func() string { return c.PanePaths(c.WindowID("dl", "w2")) })
		c.EventuallyEqual("window dir with a space inherited", c.Path("d/with space"), func() string { return c.PanePaths(c.WindowID("dl", "w3")) })

		// An attached client opens a new window in the session path; a command-line client would use its own directory.
		cc := c.AttachControl("dl")
		cc.Send("new-window -n later")
		c.WaitUntil(harness.Patience, func() bool { return c.WindowID("dl", "later") != "" })
		cc.Close()
		c.EventuallyEqual("new window opened later by user uses session dir", c.Path("d/s"), func() string { return c.PanePaths(c.WindowID("dl", "later")) })
	})

	harness.Run(t, "dirs_default_cwd", func(c *harness.Case) {
		c.Simple("dc", "here/.glaze")
		c.Cd("here")
		c.OK(c.Up(), "up")
		c.EventuallyEqual("no starting_directory uses cwd", c.Path("here"), func() string { return c.PanePaths("=dc:") })
	})

	harness.Run(t, "dirs_tilde", func(c *harness.Case) {
		c.Mkdir("home/proj")
		c.Fixture("directories/tilde.glaze")
		c.OK(c.Up(), "up with ~ in starting_directory")
		c.EventuallyEqual("~ expanded", c.Path("home/proj"), func() string { return c.PanePaths(c.WindowID("dt", "proj")) })
		c.EventuallyEqual("bare ~ expanded", c.Home, func() string { return c.PanePaths(c.WindowID("dt", "home")) })
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
		c.EventuallyEqual("relative starting_directory is relative to the profile", c.Path("prof/sub"), func() string { return c.PanePaths("=dr:") })
	})

	// From a directory that was deleted, glaze reads the current directory only where the profile needs it.
	harness.Run(t, "dirs_deleted_cwd", func(c *harness.Case) {
		if runtime.GOOS == "darwin" {
			c.T().Skip("macOS still reports the path of a deleted working directory")
		}
		c.Fixture("directories/deleted-with-directory.glaze", "prof/with.glaze")
		c.Fixture("directories/deleted-without-directory.glaze", "prof/without.glaze")
		c.Fixture("directories/deleted-path-pwd.glaze", "prof/pwd.glaze")
		// fromDeleted runs glaze in a directory that the shell removes just before the exec.
		fromDeleted := func(args ...string) *harness.Result {
			c.Mkdir("gone")
			return c.Exec(harness.Opts{Dir: "gone"}, "sh", append([]string{"-c", `rmdir "$PWD" && exec "$0" "$@"`, c.Env().Glaze}, args...)...)
		}
		with, without, pwd := c.Path("prof/with.glaze"), c.Path("prof/without.glaze"), c.Path("prof/pwd.glaze")

		c.OK(fromDeleted("format", "--validate", "--profile-path", with), "format --validate from a deleted directory")
		c.OK(fromDeleted("up", "--detached", "--socket-name", c.Socket, "--profile-path", with), "up from a deleted directory")
		c.SessionExists("session from a deleted directory", "dw")
		c.OK(fromDeleted("down", "--socket-name", c.Socket, "--profile-path", with), "down from a deleted directory")
		c.SessionGone("down from a deleted directory kills the session", "dw")

		r := fromDeleted("up", "--detached", "--socket-name", c.Socket, "--profile-path", without)
		c.Fails(r, "up without a session directory from a deleted directory")
		c.Match("the error says the session needs a directory", "no starting_directory", r.Stderr)

		r = fromDeleted("format", "--validate", "--profile-path", pwd)
		c.Fails(r, "path.pwd from a deleted directory")
		c.Match("the error names the current directory", "Current directory not available", r.Stderr)
	})

	harness.Run(t, "dirs_missing", func(c *harness.Case) {
		c.Fixture("directories/missing.glaze")
		r := c.Up()
		c.Fails(r, "missing starting_directory rejected")
		c.NoServer("missing dir")
		c.Match("missing dir diagnostic", `[Dd]irector`, r.Output())
	})

	harness.Run(t, "dirs_is_file", func(c *harness.Case) {
		c.Write("afile", "")
		c.Fixture("directories/is-file.glaze")
		c.Fails(c.Up(), "starting_directory pointing at a file rejected")
	})
}
