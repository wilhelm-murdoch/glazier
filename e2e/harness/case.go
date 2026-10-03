package harness

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
)

var caseCounter atomic.Int64

// Case is one isolated run: its own work directory, HOME, TMUX_TMPDIR and
// tmux socket. Two cases never share a tmux server, so they run in parallel.
type Case struct {
	Dir    string // the work directory, which is also the working directory of glaze
	Home   string // HOME of every process of the case
	Socket string // the -L name of the tmux server of the case

	t      T
	env    *Env
	cwd    string
	tmpdir string // TMUX_TMPDIR, short because a socket path has a length limit

	mu     sync.Mutex
	procs  []*exec.Cmd
	labels map[string]bool
}

// T is the part of *testing.T that a Case uses. The unit tests of the
// harness give a fake one.
type T interface {
	Helper()
	Name() string
	Cleanup(func())
	TempDir() string
	Logf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Skip(args ...any)
}

// Run runs fn as a parallel subtest with a new Case.
func Run(t *testing.T, name string, fn func(c *Case)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		fn(New(t))
	})
}

// New makes a Case for t and registers its cleanup. It skips t when there is
// no glaze binary, unless E2E_REQUIRE is set.
func New(t T) *Case {
	t.Helper()
	env, err := loadEnv()
	if errors.Is(err, errNoGlaze) && os.Getenv("E2E_REQUIRE") == "" {
		t.Skip("GLAZE_BIN is not set")
	}

	if err != nil {
		t.Fatal(err)
	}

	return newCase(t, env)
}

func newCase(t T, env *Env) *Case {
	t.Helper()
	dir := realDir(t, t.TempDir())
	tmpdir, err := os.MkdirTemp("/tmp", "gz") // #nosec G303 -- a socket path must stay short; /tmp keeps it short on macOS too
	if err != nil {
		t.Fatal(err)
	}

	c := &Case{
		Dir:    dir,
		Home:   filepath.Join(dir, "home"),
		Socket: fmt.Sprintf("gz%d", caseCounter.Add(1)),
		t:      t,
		env:    env,
		cwd:    dir,
		tmpdir: realDir(t, tmpdir),
		labels: map[string]bool{},
	}

	c.Mkdir("home")
	t.Cleanup(c.cleanup)
	return c
}

// T is the test of the case.
func (c *Case) T() T { return c.t }

// Env is the shared environment.
func (c *Case) Env() *Env { return c.env }

// Logf writes to the log of the case. Use it for an observation that has no
// expected result.
func (c *Case) Logf(format string, args ...any) {
	c.t.Helper()
	c.t.Logf(format, args...)
}

// Cd changes the directory in which the next commands run, like cd in a shell.
func (c *Case) Cd(rel string) { c.cwd = c.Path(rel) }

// environ is the environment of every process of the case. It starts from
// nothing, so a TMUX or GLAZE_* variable of the host never leaks in.
func (c *Case) environ(extra []string) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + c.Home,
		"TERM=xterm-256color",
		"SHELL=" + c.env.Shell,
		"TMUX_TMPDIR=" + c.tmpdir,
	}

	for _, key := range []string{"USER", "LOGNAME", "TMPDIR", "LANG", "LC_ALL", "LC_CTYPE"} {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}

	if os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" {
		env = append(env, "LANG=C.UTF-8")
	}

	return append(env, extra...)
}

func (c *Case) resolve(dir string) string {
	if dir == "" {
		return c.cwd
	}

	return c.Path(dir)
}

func (c *Case) track(cmd *exec.Cmd) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.procs = append(c.procs, cmd)
}

// cleanup stops every tmux server of the case and every process group that
// the case started, then removes its directories.
func (c *Case) cleanup() {
	// A case can make a directory unreadable; give every directory its mode back first.
	for _, root := range []string{c.Dir, c.tmpdir} {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = os.Chmod(path, dirMode) // #nosec G302 G122 -- a directory of the case, which the test removes next
			}

			return nil
		})
	}

	for _, args := range c.killArgs() {
		cmd := exec.Command(c.env.Tmux, args...) // #nosec G204 -- the tmux binary under test
		cmd.Env = c.environ(nil)
		_ = cmd.Run()
	}

	c.mu.Lock()
	for _, cmd := range c.procs {
		_ = killGroup(cmd, syscall.SIGKILL)
	}

	c.mu.Unlock()
	// A shell that tmux stops can still write its history into HOME, so retry.
	c.WaitUntil(Patience, func() bool { return os.RemoveAll(c.Dir) == nil })
	_ = os.RemoveAll(c.tmpdir)
}

// killArgs stops every server in the TMUX_TMPDIR of the case: the server of
// the case, the default server and each server on a socket in a ShortDir.
func (c *Case) killArgs() [][]string {
	if c.env.Tmux == "" {
		return nil
	}

	args := [][]string{{"-L", c.Socket, "kill-server"}, {"kill-server"}}
	_ = filepath.WalkDir(c.tmpdir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.Type()&fs.ModeSocket != 0 {
			args = append(args, []string{"-S", path, "kill-server"})
		}

		return nil
	})

	return args
}

// realDir resolves symlinks, because tmux reports a pane path with them
// resolved: /var is /private/var on macOS.
func realDir(t T, dir string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	return real
}
