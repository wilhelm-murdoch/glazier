package tmux

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var (
	ErrPaneExited     = errors.New("the pane's shell exited before its commands finished")
	ErrCommandTimeout = errors.New("the pane's commands did not finish in time")
)

// paneCheckInterval is how often a CommandRunner checks that the pane is still alive while it waits.
var paneCheckInterval = 250 * time.Millisecond

// posixShells are the shells, other than zsh, that evaluate the POSIX form of the command line.
var posixShells = []string{"sh", "bash", "dash", "ash", "ksh", "mksh", "yash", "busybox"}

// shellFamily tells the runner which form of eval and quoting the pane's shell needs.
type shellFamily int

const (
	shellPOSIX shellFamily = iota
	shellZsh
	shellFish
)

// CommandRunner runs a list of commands in a pane. It loads the commands into a tmux paste buffer, then types
// one line that makes the pane's own shell evaluate the buffer, so the line editor never sees the commands.
type CommandRunner struct {
	client  Client
	socket  string
	shell   shellFamily
	timeout time.Duration
}

// NewCommandRunner reads the socket path and the default shell of the server. A timeout of zero waits with no limit.
func (c Client) NewCommandRunner(timeout time.Duration) (*CommandRunner, error) {
	cmd := newCommand(c, "display-message", "-p", "#{socket_path}")

	c.logger.Debug(cmd.String())

	socket, err := cmd.ExecWithOutput()
	if err != nil {
		return nil, fmt.Errorf("could not read the tmux socket path: %w", err)
	}

	shell, err := c.defaultShell()
	if err != nil {
		return nil, err
	}

	family, known := shellKind(shell)
	if !known {
		c.logger.Warn("glaze does not recognise this shell, so it uses the POSIX form to run pane commands", "shell", shell)
	}

	return &CommandRunner{client: c, socket: socket, shell: family, timeout: timeout}, nil
}

// defaultShell returns the command that tmux starts in a new pane: default-command, or default-shell when that is empty.
func (c Client) defaultShell() (string, error) {
	for _, option := range []string{"default-command", "default-shell"} {
		cmd := newCommand(c, "show-options", "-gqv", option)

		c.logger.Debug(cmd.String())

		value, err := cmd.ExecWithOutput()
		if err != nil {
			return "", fmt.Errorf("could not read the tmux option `%s`: %w", option, err)
		}

		if strings.TrimSpace(value) != "" {
			return value, nil
		}
	}

	return "", nil
}

// shellKind returns the shell family that a pane command starts, and whether it names a shell that glaze knows.
func shellKind(command string) (family shellFamily, known bool) {
	for _, word := range strings.Fields(command) {
		switch name := strings.TrimPrefix(filepath.Base(word), "-"); {
		case name == "fish":
			return shellFish, true
		case name == "zsh":
			return shellZsh, true
		case slices.Contains(posixShells, name):
			return shellPOSIX, true
		}
	}

	return shellPOSIX, false
}

// Run evaluates the commands in order in the pane. It returns when every command but the last has finished,
// because the last command can run for as long as the pane exists.
func (r *CommandRunner) Run(pane string, commands []string) error {
	if len(commands) == 0 {
		return nil
	}

	// A random name stops another client from guessing the buffer, and keeps runs on the same server apart.
	token := make([]byte, 8)
	if _, err := rand.Read(token); err != nil {
		return fmt.Errorf("could not create a buffer name: %w", err)
	}

	name := "glaze-" + hex.EncodeToString(token)

	load := newCommand(r.client, "load-buffer", "-b", name, "-")

	r.client.logger.Debug(load.String())

	if err := load.ExecWithInput(r.script(name, commands)); err != nil {
		return fmt.Errorf("could not load the commands into a tmux buffer: %w", err)
	}

	if err := r.typeLine(pane, r.evalLine(name)); err != nil {
		r.deleteBuffer(name)
		return err
	}

	if len(commands) == 1 {
		return nil
	}

	return r.wait(pane, name)
}

// script returns the text that the shell evaluates: it deletes its own buffer first and signals before the last command.
// Each command gets its own eval, so a syntax error in one command cannot stop the commands after it or the signal.
func (r *CommandRunner) script(name string, commands []string) string {
	// POSIX makes an error in a special builtin such as eval fatal unless `command` runs it, but zsh's `command` skips builtins.
	eval := "command eval"
	if r.shell != shellPOSIX {
		eval = "eval"
	}

	last := len(commands) - 1

	lines := []string{r.tmuxLine("delete-buffer", "-b", name)}
	for i, command := range commands {
		if i == last && last > 0 {
			lines = append(lines, r.tmuxLine("wait-for", "-S", name))
		}

		// The leading space stops bash reading a command that starts with - as an option of eval.
		lines = append(lines, eval+" "+r.quote(" "+command))
	}

	return strings.Join(lines, "\n") + "\n"
}

// evalLine returns the line that glaze types into the pane. The leading space keeps it out of the shell history where the shell supports that.
func (r *CommandRunner) evalLine(name string) string {
	show := r.tmuxLine("show-buffer", "-b", name)
	if r.shell == shellFish {
		return fmt.Sprintf(" eval (%s | string collect)", show)
	}

	return fmt.Sprintf(` eval "$(%s)"`, show)
}

// tmuxLine returns a shell command that runs tmux against this server, by absolute path, whatever the pane's PATH and TMUX are.
func (r *CommandRunner) tmuxLine(args ...string) string {
	return strings.Join(append([]string{r.quote(r.client.tmuxPath), "-S", r.quote(r.socket)}, args...), " ")
}

// quote returns s as one single-quoted word for the pane's shell.
func (r *CommandRunner) quote(s string) string {
	if r.shell == shellFish {
		return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
	}

	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// typeLine types line into the pane as literal text, then presses Enter.
func (r *CommandRunner) typeLine(pane, line string) error {
	for _, args := range [][]string{
		{"send-keys", "-l", "-t", pane, "--", line},
		{"send-keys", "-t", pane, "Enter"},
	} {
		cmd := newCommand(r.client, args...)

		r.client.logger.Debug(cmd.String())

		if err := cmd.Exec(); err != nil {
			return fmt.Errorf("could not type the command line into pane `%s`: %w", pane, err)
		}
	}

	return nil
}

// wait blocks until the shell signals the channel, the pane dies or the timeout passes.
// The buffer name is also the channel name.
func (r *CommandRunner) wait(pane, name string) error {
	cmd := newCommand(r.client, "wait-for", name)

	r.client.logger.Debug(cmd.String())

	done := make(chan error, 1)
	go func() {
		done <- cmd.Exec()
	}()

	var deadline <-chan time.Time
	if r.timeout > 0 {
		timer := time.NewTimer(r.timeout)
		defer timer.Stop()
		deadline = timer.C
	}

	ticker := time.NewTicker(paneCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("waiting on channel `%s` failed: %w", name, err)
			}

			return nil
		case <-deadline:
			return r.giveUp(name, done, ErrCommandTimeout)
		case <-ticker.C:
			if r.paneAlive(pane) {
				continue
			}

			// The shell can signal and then exit, so give a late signal one more interval to arrive.
			select {
			case err := <-done:
				if err == nil {
					return nil
				}

				r.deleteBuffer(name)
				return ErrPaneExited
			case <-time.After(paneCheckInterval):
				return r.giveUp(name, done, ErrPaneExited)
			}
		}
	}
}

// giveUp releases the waiting client and removes the buffer in case the shell never read it.
func (r *CommandRunner) giveUp(name string, done <-chan error, reason error) error {
	cmd := newCommand(r.client, "wait-for", "-S", name)

	r.client.logger.Debug(cmd.String())

	if err := cmd.Exec(); err == nil {
		<-done
	}

	r.deleteBuffer(name)

	return reason
}

// paneAlive reports whether the pane exists and its program still runs.
// display-message prints nothing and succeeds for a missing pane, so the output must name the pane.
func (r *CommandRunner) paneAlive(pane string) bool {
	cmd := newCommand(r.client, "display-message", "-p", "-t", pane, "#{pane_id} #{pane_dead}")

	output, err := cmd.ExecWithOutput()

	return err == nil && output == pane+" 0"
}

// deleteBuffer removes the buffer. An error means the shell already removed it.
func (r *CommandRunner) deleteBuffer(name string) {
	cmd := newCommand(r.client, "delete-buffer", "-b", name)

	r.client.logger.Debug(cmd.String())

	_, _ = cmd.ExecWithOutput()
}
