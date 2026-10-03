package harness

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
)

const (
	fileMode = 0o644
	dirMode  = 0o755
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
	env, err := Load()
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

// Path makes a path relative to the work directory absolute.
func (c *Case) Path(rel ...string) string {
	p := filepath.Join(rel...)
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.Dir, p)
}

// Cd changes the directory in which the next commands run, like cd in a shell.
func (c *Case) Cd(rel string) { c.cwd = c.Path(rel) }

// Glaze runs glaze with args in the current directory of the case.
func (c *Case) Glaze(args ...string) *Result {
	c.t.Helper()
	return c.GlazeWith(Opts{}, args...)
}

// GlazeWith runs glaze with args and options.
func (c *Case) GlazeWith(o Opts, args ...string) *Result {
	c.t.Helper()
	return c.Exec(o, c.env.Glaze, args...)
}

// Up runs "glaze up --detached" against the server of the case.
func (c *Case) Up(args ...string) *Result {
	c.t.Helper()
	return c.UpWith(Opts{}, args...)
}

// UpWith runs "glaze up --detached" with options.
func (c *Case) UpWith(o Opts, args ...string) *Result {
	c.t.Helper()
	return c.GlazeWith(o, append([]string{"up", "--detached", "--socket-name", c.Socket}, args...)...)
}

// Down runs "glaze down" against the server of the case.
func (c *Case) Down(args ...string) *Result {
	c.t.Helper()
	return c.Glaze(append([]string{"down", "--socket-name", c.Socket}, args...)...)
}

// Fixture copies a file from fixtures/ into the work directory. The default
// destination is .glaze, the profile that glaze finds in its directory.
func (c *Case) Fixture(name string, dest ...string) {
	c.t.Helper()
	data, err := os.ReadFile(filepath.Join(c.env.Root, "fixtures", filepath.FromSlash(name))) // #nosec G304 -- a fixture of the module
	if err != nil {
		c.t.Fatalf("fixture: %v", err)
	}
	target := ".glaze"
	if len(dest) > 0 {
		target = dest[0]
	}
	c.Write(target, string(data))
}

// Write writes a file and makes its parent directories.
func (c *Case) Write(rel, content string) {
	c.t.Helper()
	path := c.Path(rel)
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		c.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), fileMode); err != nil { // #nosec G306 G703 -- a test file that tmux and glaze read, under the work directory
		c.t.Fatal(err)
	}
}

// Mkdir makes each directory and its parents.
func (c *Case) Mkdir(rels ...string) {
	c.t.Helper()
	for _, rel := range rels {
		if err := os.MkdirAll(c.Path(rel), dirMode); err != nil {
			c.t.Fatal(err)
		}
	}
}

// TmuxConf writes ~/.tmux.conf, which the server of the case reads at start.
func (c *Case) TmuxConf(content string) {
	c.t.Helper()
	c.Write(filepath.Join(c.Home, ".tmux.conf"), content)
}

// Read returns the content of a file, or "" when it does not exist.
func (c *Case) Read(rel string) string {
	data, err := os.ReadFile(c.Path(rel))
	if err != nil {
		return ""
	}
	return string(data)
}

// Lines returns the lines of a file joined with commas.
func (c *Case) Lines(rel string) string {
	return strings.Join(strings.Split(strings.TrimSuffix(c.Read(rel), "\n"), "\n"), ",")
}

// Exists reports whether a path exists, without following a symlink.
func (c *Case) Exists(rel string) bool {
	_, err := os.Lstat(c.Path(rel))
	return err == nil
}

// Remove removes a file or an empty directory.
func (c *Case) Remove(rel string) {
	c.t.Helper()
	if err := os.Remove(c.Path(rel)); err != nil {
		c.t.Fatal(err)
	}
}

// Symlink makes rel a symlink to target.
func (c *Case) Symlink(target, rel string) {
	c.t.Helper()
	if err := os.Symlink(target, c.Path(rel)); err != nil {
		c.t.Fatal(err)
	}
}

// Chmod changes the mode of a path.
func (c *Case) Chmod(rel string, mode fs.FileMode) {
	c.t.Helper()
	if err := os.Chmod(c.Path(rel), mode); err != nil {
		c.t.Fatal(err)
	}
}

// Mkfifo makes a named pipe.
func (c *Case) Mkfifo(rel string) {
	c.t.Helper()
	if err := syscall.Mkfifo(c.Path(rel), fileMode); err != nil {
		c.t.Fatal(err)
	}
}

// ShortDir makes a new directory with a short path, for a socket: a socket
// path has a limit of about 104 bytes. The case stops each tmux server on a
// socket in it and removes it at the end.
func (c *Case) ShortDir() string {
	c.t.Helper()
	dir, err := os.MkdirTemp(c.tmpdir, "s")
	if err != nil {
		c.t.Fatal(err)
	}
	return dir
}

// SocketPath is the path of the socket of the server of the case.
func (c *Case) SocketPath() string {
	return filepath.Join(c.tmpdir, fmt.Sprintf("tmux-%d", os.Getuid()), c.Socket)
}

// Simple writes a profile with one session, one window "w" and one pane "p".
func (c *Case) Simple(session string, dest ...string) {
	c.t.Helper()
	target := ".glaze"
	if len(dest) > 0 {
		target = dest[0]
	}
	c.Write(target, fmt.Sprintf("session {\n  name = %s\n  window {\n    name = \"w\"\n    pane {\n      name = \"p\"\n    }\n  }\n}\n", Quote(session)))
}

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
