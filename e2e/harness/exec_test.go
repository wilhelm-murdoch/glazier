package harness

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestExecResult(t *testing.T) {
	withCase(t, nil, func(c *Case) {
		r := c.Exec(Opts{Stdin: "in"}, "sh", "-c", `cat; echo out; echo err >&2; exit 3`)
		if r.Code != 3 || r.Stdout != "inout\n" || r.Stderr != "err\n" || r.TimedOut {
			t.Errorf("result %+v", r)
		}

		if r := c.Exec(Opts{}, "sh", "-c", "kill -TERM $$"); r.Code != 128+int(syscall.SIGTERM) {
			t.Errorf("a signal gives exit %d, want %d", r.Code, 128+int(syscall.SIGTERM))
		}
	})
}

func TestExecDeadline(t *testing.T) {
	withCase(t, nil, func(c *Case) {
		r := c.Exec(Opts{Timeout: 200 * time.Millisecond}, "sleep", "30")
		if !r.TimedOut || r.Duration > 5*time.Second {
			t.Errorf("sleep was not stopped: %+v", r)
		}
	})
}

func TestExecDeadlineKillsWhatIgnoresTERM(t *testing.T) {
	withCase(t, nil, func(c *Case) {
		start := time.Now()
		r := c.Exec(Opts{Timeout: 200 * time.Millisecond}, "sh", "-c", `trap "" TERM; while :; do sleep 1; done`)
		if !r.TimedOut || time.Since(start) > killDelay+5*time.Second {
			t.Errorf("the process was not killed after the delay: %+v", r)
		}
	})
}

func TestEnvironmentIsIsolated(t *testing.T) {
	t.Setenv("TMUX", "/tmp/host-tmux,1,0")
	t.Setenv("GLAZE_PATH", "/host")
	withCase(t, nil, func(c *Case) {
		env := c.Exec(Opts{Env: []string{"EXTRA=1"}}, "env").Stdout
		for _, leak := range []string{"TMUX=", "GLAZE_PATH="} {
			if strings.Contains(env, "\n"+leak) || strings.HasPrefix(env, leak) {
				t.Errorf("%s reached the case:\n%s", leak, env)
			}
		}

		for _, want := range []string{"HOME=" + c.Home, "TMUX_TMPDIR=" + c.tmpdir, "EXTRA=1", "SHELL=/bin/sh"} {
			if !strings.Contains(env, want+"\n") {
				t.Errorf("missing %s in:\n%s", want, env)
			}
		}
	})
}

func TestCdAndPaths(t *testing.T) {
	withCase(t, nil, func(c *Case) {
		c.Mkdir("sub")
		c.Cd("sub")
		if got := strings.TrimSpace(c.Exec(Opts{}, "pwd", "-P").Stdout); got != c.Path("sub") {
			t.Errorf("pwd %q, want %q", got, c.Path("sub"))
		}

		if c.Path("/abs") != "/abs" {
			t.Error("Path changed an absolute path")
		}
	})
}

func TestCleanupRemovesTheSocketDirectory(t *testing.T) {
	var tmpdir string
	withCase(t, nil, func(c *Case) { tmpdir = c.tmpdir })
	if _, err := os.Stat(tmpdir); !os.IsNotExist(err) {
		t.Errorf("%s still exists", tmpdir)
	}
}

func TestShellQuote(t *testing.T) {
	withCase(t, nil, func(c *Case) {
		for _, s := range []string{"plain", "it's", `a "b" $c \d`, "new\nline", ""} {
			if got := c.Exec(Opts{}, "sh", "-c", "printf %s "+ShellQuote(s)).Stdout; got != s {
				t.Errorf("ShellQuote(%q) gives %q in sh", s, got)
			}
		}
	})
}

func TestShortDirSocketsAreStopped(t *testing.T) {
	withCase(t, &Env{Tmux: "/usr/bin/true"}, func(c *Case) {
		dir := c.ShortDir()
		sock := filepath.Join(dir, "s")
		l, err := net.Listen("unix", sock)
		if err != nil {
			t.Fatal(err)
		}

		defer func() { _ = l.Close() }()
		found := false
		for _, args := range c.killArgs() {
			found = found || strings.Join(args, " ") == "-S "+sock+" kill-server"
		}

		if !found {
			t.Errorf("killArgs %q does not stop the server on %s", c.killArgs(), sock)
		}

		if len(sock) > 100 {
			t.Errorf("the socket path %q is too long for macOS", sock)
		}
	})
}

func TestCleanupRestoresModes(t *testing.T) {
	var dir string
	withCase(t, nil, func(c *Case) {
		dir = c.Dir
		c.Mkdir("locked/inner")
		c.Write("locked/inner/f", "x")
		c.Chmod("locked", 0)
	})

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("%s still exists: %v", dir, err)
	}
}
