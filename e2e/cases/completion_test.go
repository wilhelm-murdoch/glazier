package cases

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

// systemBash is the bash of macOS, which is 3.2 and does not read a script from source <(...).
const systemBash = "/bin/bash"

var (
	// completionShells maps each shell to the flag that makes it check a script for syntax errors without running it.
	completionShells = map[string]string{"bash": "-n", "zsh": "-n", "fish": "-n"}

	// completionLoaders are the lines that the help, the README and the man page give to load each script.
	// The shell runs the line with its arguments, then the check, which prints a match for want when the script is loaded.
	completionLoaders = []struct {
		shell string
		args  []string
		line  string
		check string
		want  string
	}{
		{"bash", []string{"--norc", "--noprofile", "-c"}, `eval "$(glaze completion bash)"`, "complete -p glaze", `-F \S+ glaze\s*$`},
		{"zsh", []string{"-f", "-c"}, `source <(glaze completion zsh)`, "print -r -- ${_comps[glaze]}", `^_glaze\s*$`},
		{"fish", []string{"-c"}, "mkdir -p ~/.config/fish/completions && glaze completion fish > ~/.config/fish/completions/glaze.fish", "complete -C 'glaze l'", `(?m)^ls\b`},
	}
)

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

	// A user copies the line from the help, so each line must load its script in a fresh shell.
	harness.Run(t, "cli_completion_load", func(c *harness.Case) {
		help := c.Glaze("completion", "--help")
		c.OK(help, "completion help")

		// The shells find glaze on PATH, as they do for a user.
		c.Mkdir("bin")
		if err := os.Symlink(c.Env().Glaze, c.Path("bin", "glaze")); err != nil {
			c.T().Fatalf("link glaze: %v", err)
		}

		opts := harness.Opts{Env: []string{"PATH=" + c.Path("bin") + string(os.PathListSeparator) + os.Getenv("PATH")}}
		for _, l := range completionLoaders {
			c.True("completion help gives the "+l.shell+" line", strings.Contains(help.Stdout, l.line), "help: %s", labelText(help.Stdout, 400))

			script := l.line + "; " + l.check
			if l.shell == "zsh" {
				// The zsh script calls compdef, so compinit runs first, as the help says.
				script = "autoload -Uz compinit; compinit -u -D; " + script
			}

			paths := shellPaths(l.shell)
			if len(paths) == 0 {
				c.Logf("%s is not installed; its line is not run", l.shell)
			}

			for i, path := range paths {
				label := l.shell
				if i > 0 {
					label = systemBash
				}

				r := c.Exec(opts, path, append(append([]string{}, l.args...), script)...)
				c.OK(r, label+" runs the documented line")
				c.Match(label+" loads the completion script", l.want, r.Stdout)
			}
		}
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

// shellPaths returns the shell on PATH and, for bash, also /bin/bash when it is a different bash.
func shellPaths(shell string) []string {
	var paths []string
	onPath, err := exec.LookPath(shell)
	if err == nil {
		paths = append(paths, onPath)
	}

	if shell != "bash" {
		return paths
	}

	if _, err := os.Stat(systemBash); err != nil || sameFile(onPath, systemBash) {
		return paths
	}

	return append(paths, systemBash)
}

// sameFile reports whether two paths name the same file after symlinks.
func sameFile(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)

	return errA == nil && errB == nil && ra == rb
}
