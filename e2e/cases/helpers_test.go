package cases

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// The exit codes of glaze.
const (
	exitFailure        = 1 // a run that failed
	exitUsage          = 2 // a wrong flag or argument
	exitInvalidProfile = 3 // a profile that does not parse or validate
	exitUnreachable    = 4 // a tmux server that glaze cannot reach
	exitDifferences    = 5 // diff: the session differs from the profile or does not run
)

// hangTimeout is the deadline of a glaze command that can hang. It is shorter
// than the harness default, so that a hang fails the case quickly.
const hangTimeout = 10 * time.Second

// registryHome is the profile registry of a case, relative to the work
// directory: `.config/glaze` under the HOME of the case.
const registryHome = "home/.config/glaze"

// fileLines returns a function that reads the lines of a file in the work
// directory, joined with commas. Give it to EventuallyEqual.
func fileLines(c *harness.Case, rel string) func() string {
	return func() string { return c.Lines(rel) }
}

// firstLine returns the first line of s.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// labelText shows text in a label: on one line, a tab as \t and at most n
// characters, so that a label stays readable and stable.
func labelText(s string, n int) string {
	s = strings.NewReplacer("\n", " ", "\t", `\t`).Replace(s)
	if runes := []rune(s); len(runes) > n {
		return string(runes[:n])
	}

	return s
}

// workFiles returns the names of the files in the work directory that match
// a glob pattern, sorted and joined with commas. The pattern also matches a
// file whose name starts with a dot.
func workFiles(c *harness.Case, pattern string) string {
	paths, err := filepath.Glob(c.Path(pattern))
	if err != nil {
		c.T().Fatal(err)
	}

	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
	}

	slices.Sort(names)
	return strings.Join(names, ",")
}

// modeOf returns the permissions of a file in the form of ls, for example
// -rw-------, without following a symlink.
func modeOf(c *harness.Case, rel string) string {
	info, err := os.Lstat(c.Path(rel))
	if err != nil {
		return ""
	}

	return info.Mode().String()
}

// rawLayout returns the layout string of a window with n side-by-side panes.
// It stops the server, so the next up starts a new server and NoServer can pass.
func rawLayout(c *harness.Case, n int) string {
	c.TmuxSetup("new-session", "-d", "-s", "ref")
	for i := 1; i < n; i++ {
		c.TmuxSetup("split-window", "-h", "-t", "=ref:")
	}

	raw := c.Tmux("display-message", "-p", "-t", "=ref:", "#{window_layout}")
	c.KillServer()
	return raw
}

// isSymlink reports whether a path in the work directory is a symlink.
func isSymlink(c *harness.Case, rel string) bool {
	info, err := os.Lstat(c.Path(rel))
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// xdgConfigHome returns the options of a command with XDG_CONFIG_HOME set to dir.
func xdgConfigHome(dir string) harness.Opts {
	return harness.Opts{Env: []string{"XDG_CONFIG_HOME=" + dir}}
}

// glazeRegistry returns the options of a command with GLAZE_REGISTRY set to dir.
func glazeRegistry(dir string) harness.Opts {
	return harness.Opts{Env: []string{"GLAZE_REGISTRY=" + dir}}
}

// fromDeleted runs glaze with args in a directory that the shell removes just
// before the exec, so glaze starts without a current directory.
func fromDeleted(c *harness.Case, args ...string) *harness.Result {
	c.T().Helper()
	c.Mkdir("gone")
	return c.Exec(harness.Opts{Dir: "gone"}, "sh", append([]string{"-c", `rmdir "$PWD" && exec "$0" "$@"`, c.Env().Glaze}, args...)...)
}
