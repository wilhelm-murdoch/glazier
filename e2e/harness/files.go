package harness

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	fileMode = 0o644
	dirMode  = 0o755

	// defaultProfile is the profile that glaze finds in its working directory.
	defaultProfile = ".glaze"

	// simpleProfile is the profile that Simple writes; %s is the quoted session name.
	simpleProfile = `session {
  name = %s
  window {
    name = "w"
    pane {
      name = "p"
    }
  }
}
`
)

// Path makes a path relative to the work directory absolute.
func (c *Case) Path(rel ...string) string {
	p := filepath.Join(rel...)
	if filepath.IsAbs(p) {
		return p
	}

	return filepath.Join(c.Dir, p)
}

// Fixture copies a file from fixtures/ into the work directory. The default
// destination is .glaze, the profile that glaze finds in its directory.
func (c *Case) Fixture(name string, dest ...string) {
	c.t.Helper()
	data, err := os.ReadFile(filepath.Join(c.env.Root, "fixtures", filepath.FromSlash(name))) // #nosec G304 -- a fixture of the module
	if err != nil {
		c.t.Fatalf("fixture: %v", err)
	}

	c.Write(profilePath(dest), string(data))
}

// Simple writes a profile with one session, one window "w" and one pane "p".
// The default destination is .glaze.
func (c *Case) Simple(session string, dest ...string) {
	c.t.Helper()
	c.Write(profilePath(dest), fmt.Sprintf(simpleProfile, Quote(session)))
}

// profilePath returns the optional destination of Fixture and Simple.
func profilePath(dest []string) string {
	if len(dest) > 0 {
		return dest[0]
	}

	return defaultProfile
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

// ShortSocket returns the path of a socket in a new ShortDir, for
// --socket-path. The case stops the server on it at the end.
func (c *Case) ShortSocket() string {
	c.t.Helper()
	return filepath.Join(c.ShortDir(), "s.sock")
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
