package cases

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// formatOldTime is 2001-01-01 00:00:00 UTC, an mtime that no write of the case can give.
var formatOldTime = time.Unix(978307200, 0)

// format rewrites a profile in the canonical format and validates it with --validate.
func TestFormat(t *testing.T) {
	harness.Run(t, "fmt_stdout", func(c *harness.Case) {
		c.Fixture("malformed/format-unformatted.glaze")
		orig := c.Read(".glaze")
		r := c.Glaze("format", "--stdout")
		c.OK(r, "format --stdout")
		c.Equal("--stdout leaves file untouched", orig, c.Read(".glaze"))
		c.Match("canonical indentation", "\n  name = \"fm\"", r.Stdout)
		c.Match("comments preserved", "# leading comment", r.Stdout)
		c.Match("trailing comment preserved", "# trailing comment", r.Stdout)
		c.NoMatch("no log noise on stdout", "INF|WRN", r.Stdout)
		c.Golden("format --stdout output", "format/stdout.glaze", r.Stdout)
		c.OK(c.Glaze("format"), "format in place")
		once := c.Read(".glaze")
		c.Glaze("format")
		c.Equal("format is idempotent", once, c.Read(".glaze"))
		c.Equal("in-place equals --stdout", once, c.Glaze("format", "--stdout").Stdout)
	})

	harness.Run(t, "fmt_validate_invalid", func(c *harness.Case) {
		c.Fixture("malformed/format-invalid-layout.glaze")
		orig := c.Read(".glaze")
		c.Fails(c.Glaze("format", "--validate"), "--validate with invalid layout")
		c.Equal("file untouched on validation error", orig, c.Read(".glaze"))
	})

	harness.Run(t, "fmt_syntax_error", func(c *harness.Case) {
		c.Fixture("malformed/format-syntax-error.glaze")
		orig := c.Read(".glaze")
		c.Fails(c.Glaze("format"), "format on syntax error")
		c.Equal("file untouched on syntax error", orig, c.Read(".glaze"))
	})

	harness.Run(t, "fmt_validate_vars", func(c *harness.Case) {
		c.Fixture("variables/typed.glaze")
		c.Fails(c.Glaze("format", "--validate"), "--validate without required var")
		c.OK(c.Glaze("format", "--validate", "--var", "fixer=x"), "--validate with required var")
		c.Fails(c.Glaze("format", "--validate", "--var", "fixer=x", "--var", "count=abc"), "--validate with bad typed var")
	})

	harness.Run(t, "fmt_multiple_errors", func(c *harness.Case) {
		c.Fixture("format/multiple-errors.glaze")
		r := c.Glaze("format", "--validate")
		c.Fails(r, "--validate multi-error")
		n := 0
		for _, line := range strings.Split(r.Output(), "\n") {
			if strings.Contains(line, "Error") {
				n++
			}
		}
		c.True("all errors reported in one run", n >= 3, "saw %d: %s", n, r.Output())
		c.NoMatch("diagnostics show source snippets", "source code not available", r.Output())
		// The order of the three errors changes from run to run, so the output is no golden file.
		c.Logf("diagnostic rendering:\n%s", r.Output())
	})

	harness.Run(t, "fmt_perms_symlink", func(c *harness.Case) {
		c.Write("real.glaze", formatUnformatted("fp"))
		c.Chmod("real.glaze", 0o600)
		c.Symlink("real.glaze", ".glaze")
		c.Simple("fp", "want.glaze")
		c.OK(c.Glaze("format"), "format through symlink")
		info, err := os.Lstat(c.Path(".glaze"))
		c.True("symlink preserved", err == nil && info.Mode()&os.ModeSymlink != 0, "format replaced the symlink with a regular file")
		c.Equal("permissions preserved", "-rw-------", formatMode(c, "real.glaze"))
		c.Equal("target formatted through the symlink", c.Read("want.glaze"), c.Read("real.glaze"))
	})

	harness.Run(t, "fmt_unchanged", func(c *harness.Case) {
		c.Simple("fu")
		c.Glaze("format")
		if err := os.Chtimes(c.Path(".glaze"), formatOldTime, formatOldTime); err != nil {
			c.T().Fatal(err)
		}
		c.OK(c.Glaze("format"), "format of a formatted profile")
		info, err := os.Stat(c.Path(".glaze"))
		if err != nil {
			c.T().Fatal(err)
		}
		c.Equal("a formatted profile keeps its mtime", "978307200", fmt.Sprint(info.ModTime().Unix()))
	})

	harness.Run(t, "fmt_write_fails", func(c *harness.Case) {
		c.Write(".glaze", formatUnformatted("fw"))
		orig := c.Read(".glaze")
		// A file size limit of zero makes every write fail, like a full disk.
		r := c.Exec(harness.Opts{}, "sh", "-c", `ulimit -f 0; exec "$0" format`, c.Env().Glaze)
		c.Fails(r, "format with a failed write")
		c.Equal("profile kept whole after a failed write", orig, c.Read(".glaze"))
		entries, err := os.ReadDir(c.Dir)
		if err != nil {
			c.T().Fatal(err)
		}
		var temps []string
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".tmp") {
				temps = append(temps, e.Name())
			}
		}
		c.Equal("no temporary file left", "", strings.Join(temps, ","))
	})

	harness.Run(t, "fmt_fifo", func(c *harness.Case) {
		c.Mkfifo("p.glaze")
		r := c.Glaze("format", "--profile-path", "p.glaze")
		c.Fails(r, "format of a FIFO in place")
		c.Match("FIFO refusal says to use --stdout", "use --stdout", r.Stderr)
	})

	harness.Run(t, "fmt_dev_stdin", func(c *harness.Case) {
		r := c.Glaze("format", "--profile-path", "/dev/stdin")
		c.Fails(r, "format of /dev/stdin in place")
		c.Match("/dev/stdin refusal says to use --stdout", "use --stdout", r.Stderr)
	})

	harness.Run(t, "fmt_process_substitution", func(c *harness.Case) {
		c.Simple("fps")
		r := c.Exec(harness.Opts{}, "bash", "-c", `"$0" format --stdout --profile-path <(cat .glaze)`, c.Env().Glaze)
		c.OK(r, "format --stdout of a process substitution")
		c.Match("process substitution formatted", `name = "fps"`, r.Stdout)
	})

	harness.Run(t, "fmt_no_tmux_needed", func(c *harness.Case) {
		c.Simple("fn")
		c.OK(c.Glaze("format", "--validate"), "format --validate works")
		c.NoServer("format")
	})

	// A user cannot write a read-only file. root can, so as root the case only logs the result.
	harness.Run(t, "fmt_readonly", func(c *harness.Case) {
		c.Write(".glaze", formatUnformatted("fr"))
		c.Chmod(".glaze", 0o444)
		orig := c.Read(".glaze")
		r := c.Glaze("format")
		c.Equal("read-only file keeps its mode", "-r--r--r--", formatMode(c, ".glaze"))
		if os.Geteuid() != 0 {
			c.Fails(r, "format refuses a read-only file")
			c.Equal("read-only file untouched", orig, c.Read(".glaze"))
			c.Match("the refusal names the cause", "permission denied", r.Stderr)
		} else {
			c.Logf("format on read-only file as root: exit %d", r.Code)
		}
	})
}

// formatUnformatted returns the profile of Case.Simple with spacing that format changes.
func formatUnformatted(session string) string {
	return fmt.Sprintf("session   {\n  name=%s\n  window {\n    name = \"w\"\n    pane {\n      name = \"p\"\n    }\n  }\n}\n", harness.Quote(session))
}

// formatMode returns the permission bits of a file, for example -rw-------.
func formatMode(c *harness.Case, rel string) string {
	info, err := os.Stat(c.Path(rel))
	if err != nil {
		return err.Error()
	}
	return info.Mode().Perm().String()
}
