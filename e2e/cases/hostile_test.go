package cases

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

const (
	// hostileLabelLength is the number of characters of a session name that a label shows.
	hostileLabelLength = 20

	// hostileSessionProfile has the session name and the session commands as %s.
	hostileSessionProfile = `session {
  name     = %s
  commands = %s
  window {
    name = "w"
    pane {
      name = "p"
    }
    pane {
      name = "q"
    }
  }
}`

	// hostileWindowProfile has the name of the first window as %s. automatic-rename is off, so tmux keeps the name.
	hostileWindowProfile = `session {
  name = "hostile-window"
  window {
    name   = %s
    layout = "even-horizontal"
    focus  = true
    options = {
      "automatic-rename" = "off"
    }
    pane {
      name = "p"
    }
    pane {
      name = "q"
    }
  }
  window {
    name = "other"
    pane {}
  }
}`

	// hostilePaneProfile has the name of the first pane as %s.
	hostilePaneProfile = `session {
  name = "hostile-pane"
  window {
    name = "w"
    pane {
      name  = %s
      focus = true
    }
    pane {
      name = "z"
    }
  }
}`

	// hostileDirProfile has the starting directory of the session and of each pane as %[1]s.
	hostileDirProfile = `session {
  name               = "hostile-dir"
  starting_directory = %[1]s
  window {
    name = "w"
    pane {
      starting_directory = %[1]s
    }
    pane {
      starting_directory = %[1]s
    }
  }
}`
)

var (
	// hostileSessionRenamed matches each character that glaze replaces with - in a session name.
	hostileSessionRenamed = regexp.MustCompile(`[.:\\$\x00-\x1f\x7f]`)

	// The names that each case tries. Each one has a character that tmux, HCL or a shell treat as special.
	hostileSessionNames = []string{"my session", "a.b", "a:b", "semi;colon", "semi;", "hash#tag", "ünïcødé", "-dash", "pct%s", "brace{x}", "tab\tx", "x=y", strings.Repeat("a", 200)}
	hostileWindowNames  = []string{"w;1", "w;", "w:1", "w.1", "w 1", "#[fg=red]x", "#{session_name}", "ü"}
	hostilePaneNames    = []string{"p;1", "p;", "p|1", "p 1", "#{pane_id}"}
	hostileDirNames     = []string{"semi;dir", "dir;", "sp ace", "ünï", "quote'd"}
)

// glaze keeps a session, window, pane or directory name with characters that tmux or a shell treat as special.
// Each name gets a new server, because each round stops the server at its end.
func TestHostileNames(t *testing.T) {
	harness.Run(t, "hostile_session", func(c *harness.Case) {
		for _, name := range hostileSessionNames {
			hostileSession(c, name)
		}
	})

	harness.Run(t, "hostile_empty_name", func(c *harness.Case) {
		c.Fixture("hostile/empty-name.glaze")
		r := c.Up()
		c.Fails(r, "empty session name rejected")
		c.NoServer("empty session name")
		c.Match("empty session name diagnostic", "must not be empty", r.Stderr)
	})

	harness.Run(t, "hostile_window", func(c *harness.Case) {
		for _, name := range hostileWindowNames {
			hostileWindow(c, name)
		}
	})

	harness.Run(t, "hostile_pane", func(c *harness.Case) {
		for _, name := range hostilePaneNames {
			hostilePane(c, name)
		}
	})

	harness.Run(t, "hostile_dir", func(c *harness.Case) {
		for _, name := range hostileDirNames {
			hostileDir(c, name)
		}
	})
}

// hostileSession runs up twice and down with a session name. The session commands make up type into
// a pane of the session, so a name that breaks the target makes up fail or hang.
func hostileSession(c *harness.Case, name string) {
	defer c.KillServer()
	short := labelText(name, hostileLabelLength)
	c.Write(".glaze", fmt.Sprintf(hostileSessionProfile, harness.Quote(name), harness.List("echo ok > "+c.Path("o"), "echo ok2 >> "+c.Path("o"))))
	r := c.UpWith(harness.Opts{Timeout: hangTimeout})
	// glaze replaces the characters that tmux rewrites in a session name, and each control character, with -.
	want := hostileSessionRenamed.ReplaceAllString(name, "-")
	if want != name {
		c.Match(fmt.Sprintf("warns about the renamed session [%s]", short), "replacing them with hyphens", r.Stderr)
	}

	got := strings.Join(c.Sessions(), "\n")
	c.True(fmt.Sprintf("session name [%s] works", short), r.Succeeded() && got == want,
		"tmux has %q, windows %q; %s", got, c.Tmux("list-windows", "-a", "-F", "#W"), r.Describe())
	if !r.Succeeded() {
		return
	}

	r = c.UpWith(harness.Opts{Timeout: hangTimeout})
	c.True(fmt.Sprintf("second up idempotent [%s]", short), r.Succeeded() && len(c.Sessions()) == 1,
		"sessions %q; %s", c.Sessions(), r.Describe())
	r = c.Down()
	c.True(fmt.Sprintf("down removes [%s]", short), len(c.Sessions()) == 0, "left %q; %s", c.Sessions(), r.Describe())
}

// hostileWindow runs up and save with a window name.
func hostileWindow(c *harness.Case, name string) {
	defer c.KillServer()
	c.Write(".glaze", fmt.Sprintf(hostileWindowProfile, harness.Quote(name)))
	r := c.Up()
	windows := c.WindowNames("hostile-window")
	c.True(fmt.Sprintf("window name [%s] works", name), r.Succeeded() && windows == name+",other", "windows %q; %s", windows, r.Describe())
	if !r.Succeeded() {
		return
	}

	r = c.Save("--session", "hostile-window", "--stdout")
	c.True(fmt.Sprintf("save with window [%s]", name), r.Succeeded(), "%s", r.Describe())
}

// hostilePane runs up and save with a pane name.
func hostilePane(c *harness.Case, name string) {
	defer c.KillServer()
	c.Write(".glaze", fmt.Sprintf(hostilePaneProfile, harness.Quote(name)))
	r := c.Up()
	titles := c.PaneTitles("=hostile-pane:w")
	c.True(fmt.Sprintf("pane name [%s] works", name), r.Succeeded() && titles == name+",z", "titles %q; %s", titles, r.Describe())
	if !r.Succeeded() {
		return
	}

	r = c.Save("--session", "hostile-pane", "--stdout")
	c.True(fmt.Sprintf("save with pane [%s]", name), r.Succeeded() && strings.Contains(r.Stdout, "focus"), "%s", r.Describe())
}

// hostileDir runs up, ls and save with a starting directory.
func hostileDir(c *harness.Case, name string) {
	defer c.KillServer()
	dir := c.Path(name)
	c.Mkdir(name)
	c.Write(".glaze", fmt.Sprintf(hostileDirProfile, harness.Quote(dir)))
	r := c.Up()
	c.True(fmt.Sprintf("dir [%s] up", name), r.Succeeded(), "%s", r.Describe())
	c.EventuallyPanePaths(fmt.Sprintf("dir [%s] paths", name), dir+","+dir, "=hostile-dir:w")
	r = c.Ls()
	c.Match(fmt.Sprintf("ls with dir [%s]", name), "hostile-dir +1 +"+regexp.QuoteMeta(dir), r.Stdout)
	r = c.Save("--session", "hostile-dir", "--stdout")
	c.True(fmt.Sprintf("save with dir [%s]", name), r.Succeeded() && strings.Contains(r.Stdout, dir), "%s", r.Describe())
}
