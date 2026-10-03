package cases

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// hostileSessionRenamed matches each character that glaze replaces with - in a session name.
var hostileSessionRenamed = regexp.MustCompile(`[.:\\$\x00-\x1f\x7f]`)

const (
	hostileUpTimeout = 10 * time.Second

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
}
`
	hostileWindowProfile = `session {
  name = "hw"
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
}
`
	hostilePaneProfile = `session {
  name = "hp"
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
}
`
	hostileDirProfile = `session {
  name               = "hd"
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
}
`
)

func TestHostileNames(t *testing.T) {
	// Each name gets a new server: the case stops the server after each name.
	harness.Run(t, "hostile_session", func(c *harness.Case) {
		names := []string{"my session", "a.b", "a:b", "semi;colon", "semi;", "hash#tag", "ünïcødé", "-dash", "pct%s", "brace{x}", "tab\tx", "x=y", strings.Repeat("a", 200)}
		for _, n := range names {
			short := hostileShort(n, 20)
			c.Write(".glaze", fmt.Sprintf(hostileSessionProfile, harness.Quote(n), harness.List("echo ok > "+c.Path("o"), "echo ok2 >> "+c.Path("o"))))
			r := c.UpWith(harness.Opts{Timeout: hostileUpTimeout})
			// glaze replaces the characters that tmux rewrites in a session name, and each control character, with -.
			want := hostileSessionRenamed.ReplaceAllString(n, "-")
			if want != n {
				c.Match(fmt.Sprintf("warns about the renamed session [%s]", short), "replacing them with hyphens", r.Output())
			}
			got := strings.Join(c.Sessions(), "\n")
			c.True(fmt.Sprintf("session name [%s] works", short), r.Code == 0 && !r.TimedOut && got == want,
				"tmux has %q, windows %q; %s", got, c.Tmux("list-windows", "-a", "-F", "#W"), r.Describe())
			if r.Code == 0 && !r.TimedOut {
				r = c.UpWith(harness.Opts{Timeout: hostileUpTimeout})
				c.True(fmt.Sprintf("second up idempotent [%s]", short), r.Code == 0 && !r.TimedOut && len(c.Sessions()) == 1,
					"sessions %q; %s", c.Sessions(), r.Describe())
				r = c.Down()
				c.True(fmt.Sprintf("down removes [%s]", short), len(c.Sessions()) == 0, "left %q; %s", c.Sessions(), r.Describe())
			}
			c.KillServer()
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
		for _, n := range []string{"w;1", "w;", "w:1", "w.1", "w 1", "#[fg=red]x", "#{session_name}", "ü"} {
			c.Write(".glaze", fmt.Sprintf(hostileWindowProfile, harness.Quote(n)))
			r := c.Up()
			windows := c.Tmux("list-windows", "-t", "=hw", "-F", "#{window_name}")
			c.True(fmt.Sprintf("window name [%s] works", n), r.Code == 0 && firstLine(windows) == n && len(strings.Split(windows, "\n")) == 2,
				"windows %q; %s", windows, r.Describe())
			if r.Code == 0 {
				r = c.Glaze("save", "--session", "hw", "--stdout", "--socket-name", c.Socket)
				c.True(fmt.Sprintf("save with window [%s]", n), r.Code == 0 && !r.TimedOut, "%s", r.Describe())
			}
			c.KillServer()
		}
	})

	harness.Run(t, "hostile_pane", func(c *harness.Case) {
		for i, n := range []string{"p;1", "p;", "p|1", "p 1", "#{pane_id}"} {
			c.Write(".glaze", fmt.Sprintf(hostilePaneProfile, harness.Quote(n)))
			r := c.Up()
			titles := c.Tmux("list-panes", "-t", "=hp:w", "-F", "#{pane_title}")
			c.True(fmt.Sprintf("pane name [%s] works", n), r.Code == 0 && firstLine(titles) == n, "titles %q; %s", titles, r.Describe())
			if r.Code == 0 {
				saved := fmt.Sprintf("s%d.glaze", i)
				r = c.Glaze("save", "--session", "hp", "--profile-path", saved, "--socket-name", c.Socket)
				c.True(fmt.Sprintf("save with pane [%s]", n), r.Code == 0 && strings.Contains(c.Read(saved), "focus"),
					"file %q; %s", c.Read(saved), r.Describe())
			}
			c.KillServer()
		}
	})

	harness.Run(t, "hostile_dir", func(c *harness.Case) {
		for i, n := range []string{"semi;dir", "dir;", "sp ace", "ünï", "quote'd"} {
			dir := c.Path(n)
			c.Mkdir(n)
			c.Write(".glaze", fmt.Sprintf(hostileDirProfile, harness.Quote(dir)))
			r := c.Up()
			c.True(fmt.Sprintf("dir [%s] up", n), r.Code == 0 && !r.TimedOut, "%s", r.Describe())
			c.Equal(fmt.Sprintf("dir [%s] paths", n), dir+","+dir, c.PanePaths("=hd:w"))
			r = c.Glaze("ls", "--socket-name", c.Socket)
			c.Match(fmt.Sprintf("ls with dir [%s]", n), "hd +1 +"+regexp.QuoteMeta(dir), r.Stdout)
			saved := fmt.Sprintf("s%d.glaze", i)
			r = c.Glaze("save", "--session", "hd", "--profile-path", saved, "--socket-name", c.Socket)
			c.True(fmt.Sprintf("save with dir [%s]", n), r.Code == 0 && strings.Contains(c.Read(saved), dir),
				"file %q; %s", c.Read(saved), r.Describe())
			c.KillServer()
		}
	})
}

// hostileShort returns the first n characters of a name for a label, with a tab shown as \t.
func hostileShort(name string, n int) string {
	runes := []rune(name)
	if len(runes) > n {
		runes = runes[:n]
	}
	return strings.ReplaceAll(string(runes), "\t", `\t`)
}
