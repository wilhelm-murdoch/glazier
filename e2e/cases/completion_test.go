package cases

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// completionShells maps each shell to the flag that makes it check a script for syntax errors without running it.
var completionShells = map[string]string{"bash": "-n", "zsh": "-n", "fish": "-n"}

// TestCompletion checks the completion scripts and the answers that glaze gives them.
func TestCompletion(t *testing.T) {
	// A script that a shell cannot parse breaks every prompt that loads it, so each one goes through its shell's syntax check.
	harness.Run(t, "cli_completion_scripts", func(c *harness.Case) {
		for shell, flag := range completionShells {
			r := c.Glaze("completion", shell)
			c.OK(r, "completion "+shell)
			c.Match("completion "+shell+" registers glaze", `glaze`, r.Stdout)
			c.Write("completion."+shell, r.Stdout)

			path, err := exec.LookPath(shell)
			if err != nil {
				c.Logf("%s is not installed; the script is not checked", shell)
				continue
			}

			c.OK(c.Exec(harness.Opts{}, path, flag, c.Path("completion."+shell)), shell+" parses its script")
		}

		c.ExitCode(c.Glaze("completion", "nope"), "completion for an unknown shell is a usage error (exit 2)", exitUsage)
	})

	// The scripts run glaze with --generate-shell-completion and read one `token:description` line per candidate.
	harness.Run(t, "cli_completion_candidates", func(c *harness.Case) {
		r := c.Glaze("--generate-shell-completion")
		c.OK(r, "candidates for the root command")
		for _, sub := range []string{"up", "down", "ls", "format", "save", "completion"} {
			c.True("root candidates include "+sub, strings.Contains(r.Stdout, "\n"+sub+":") || strings.HasPrefix(r.Stdout, sub+":"), "stdout: %s", labelText(r.Stdout, 200))
		}

		r = c.Glaze("up", "--p", "--generate-shell-completion")
		c.OK(r, "candidates for a flag of up")
		c.Match("flag candidates include --profile-path", `(?m)^--profile-path:`, r.Stdout)

		r = c.Glaze("completion", "--generate-shell-completion")
		c.OK(r, "candidates for completion")
		c.Match("completion candidates include fish", `(?m)^fish:`, r.Stdout)
	})
}
