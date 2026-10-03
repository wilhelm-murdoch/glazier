package cases

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

const (
	// unformattedFixture has spacing that format changes.
	unformattedFixture = "malformed/unformatted.glaze"

	// multipleErrors is the number of errors in format/multiple-errors.glaze.
	multipleErrors = 3
)

// formatOldTime is 2001-01-01 00:00:00 UTC, an mtime that no write of the case can give.
var formatOldTime = time.Unix(978307200, 0)

// format rewrites a profile in the canonical format and validates it with --validate.
func TestFormat(t *testing.T) {
	harness.Run(t, "fmt_stdout", func(c *harness.Case) {
		c.Fixture(unformattedFixture)
		before := c.Read(".glaze")
		r := c.Glaze("format", "--stdout")
		c.OK(r, "format --stdout")
		c.Equal("--stdout leaves file untouched", before, c.Read(".glaze"))
		c.Match("canonical indentation", "\n  name = \"unformatted\"", r.Stdout)
		c.Match("comments preserved", "# leading comment", r.Stdout)
		c.Match("trailing comment preserved", "# trailing comment", r.Stdout)
		c.NoMatch("no log noise on stdout", "INF|WRN", r.Stdout)
		c.Golden("format --stdout output", "format/stdout.glaze", r.Stdout)
		c.OK(c.Glaze("format"), "format in place")
		once := c.Read(".glaze")
		c.Must(c.Glaze("format"), "format a second time")
		c.Equal("format is idempotent", once, c.Read(".glaze"))
		c.Equal("in-place equals --stdout", once, c.Glaze("format", "--stdout").Stdout)
	})

	harness.Run(t, "fmt_validate_invalid", func(c *harness.Case) {
		c.Fixture("diagnostics/invalid-layout.glaze")
		before := c.Read(".glaze")
		c.Fails(c.Glaze("format", "--validate"), "--validate with invalid layout")
		c.Equal("file untouched on validation error", before, c.Read(".glaze"))
	})

	harness.Run(t, "fmt_syntax_error", func(c *harness.Case) {
		c.Fixture("malformed/syntax-missing-value.glaze")
		before := c.Read(".glaze")
		c.Fails(c.Glaze("format"), "format on syntax error")
		c.Equal("file untouched on syntax error", before, c.Read(".glaze"))
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
		errorCount := strings.Count(r.Stderr, "Error:")
		c.True("all errors reported in one run", errorCount >= multipleErrors, "saw %d errors, want %d: %s", errorCount, multipleErrors, r.Stderr)
		c.NoMatch("diagnostics show source snippets", "source code not available", r.Stderr)
		c.Golden("diagnostics in the order of the profile", "format/multiple-errors.txt", r.Stderr)
	})

	// The formatted file must equal the golden output of format --stdout for the same fixture.
	harness.Run(t, "fmt_perms_symlink", func(c *harness.Case) {
		c.Fixture(unformattedFixture, "real.glaze")
		c.Chmod("real.glaze", 0o600)
		c.Symlink("real.glaze", ".glaze")
		c.OK(c.Glaze("format"), "format through symlink")
		c.True("symlink preserved", isSymlink(c, ".glaze"), "format replaced the symlink with a regular file")
		c.Equal("permissions preserved", "-rw-------", modeOf(c, "real.glaze"))
		c.Golden("target formatted through the symlink", "format/stdout.glaze", c.Read("real.glaze"))
	})

	harness.Run(t, "fmt_unchanged", func(c *harness.Case) {
		c.Simple("fu")
		c.Must(c.Glaze("format"), "format the profile once")
		if err := os.Chtimes(c.Path(".glaze"), formatOldTime, formatOldTime); err != nil {
			c.T().Fatal(err)
		}

		c.OK(c.Glaze("format"), "format of a formatted profile")
		info, err := os.Stat(c.Path(".glaze"))
		if err != nil {
			c.T().Fatal(err)
		}

		c.Equal("a formatted profile keeps its mtime", fmt.Sprint(formatOldTime.Unix()), fmt.Sprint(info.ModTime().Unix()))
	})

	// glaze writes a hidden .tmp file next to the profile and renames it over the profile.
	harness.Run(t, "fmt_write_fails", func(c *harness.Case) {
		c.Fixture(unformattedFixture)
		before := c.Read(".glaze")
		// A file size limit of zero makes every write fail, like a full disk.
		r := c.Exec(harness.Opts{}, "sh", "-c", `ulimit -f 0; exec "$0" format`, c.Env().Glaze)
		c.ExitCode(r, "a failed write is a failure, not an invalid profile (exit 1)", exitFailure)
		c.Match("a failed write says that glaze could not write", "could not write", r.Stderr)
		c.Equal("profile kept whole after a failed write", before, c.Read(".glaze"))
		c.Equal("no temporary file left", "", workFiles(c, "*.tmp"))
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

	// Process substitution needs bash.
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
		c.Fixture(unformattedFixture)
		c.Chmod(".glaze", 0o444)
		before := c.Read(".glaze")
		r := c.Glaze("format")
		c.Equal("read-only file keeps its mode", "-r--r--r--", modeOf(c, ".glaze"))
		if os.Geteuid() != 0 {
			c.ExitCode(r, "format refuses a read-only file with a failure (exit 1)", exitFailure)
			c.Equal("read-only file untouched", before, c.Read(".glaze"))
			c.Match("the refusal names the cause", "permission denied", r.Stderr)
		} else {
			c.Logf("format on read-only file as root: exit %d", r.Code)
		}
	})
}
