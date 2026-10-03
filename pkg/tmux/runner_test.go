package tmux

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// runnerClient returns a client with a tmux path, because the runner writes the path into the pane's command line.
func runnerClient() Client {
	return Client{logger: discardLogger, tmuxPath: "/usr/bin/tmux"}
}

// testRunner returns a runner for a POSIX shell on the socket /tmp/tmux-1000/default.
func testRunner(timeout time.Duration) *CommandRunner {
	return &CommandRunner{client: runnerClient(), socket: "/tmp/tmux-1000/default", timeout: timeout}
}

// fastPaneChecks makes the runner check the pane often, so tests that wait for a dead pane finish quickly.
func fastPaneChecks(t *testing.T) {
	previous := paneCheckInterval
	paneCheckInterval = 5 * time.Millisecond
	t.Cleanup(func() { paneCheckInterval = previous })
}

// waitFor runs runner.wait and fails the test when it does not return within five seconds.
func waitFor(t *testing.T, runner *CommandRunner) error {
	t.Helper()

	done := make(chan error, 1)
	go func() { done <- runner.wait("%1", "glaze-x") }()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("wait did not return")
		return nil
	}
}

func TestShellKind(t *testing.T) {
	cases := []struct {
		command string
		family  shellFamily
		known   bool
	}{
		{"/bin/zsh", shellZsh, true},
		{"/usr/local/bin/bash", shellPOSIX, true},
		{"/bin/sh", shellPOSIX, true},
		{"/bin/busybox sh", shellPOSIX, true},
		{"/usr/bin/fish", shellFish, true},
		{"-fish", shellFish, true},
		{"exec /opt/homebrew/bin/fish -l", shellFish, true},
		{"reattach-to-user-namespace -l zsh", shellZsh, true},
		{"/usr/bin/nu", shellPOSIX, false},
		{"", shellPOSIX, false},
	}

	for _, c := range cases {
		t.Run(c.command, func(t *testing.T) {
			family, known := shellKind(c.command)
			assert.Equal(t, c.family, family)
			assert.Equal(t, c.known, known)
		})
	}
}

func TestNewCommandRunner(t *testing.T) {
	t.Run("reads the socket path and the default shell", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})
		rec.On("show-options", fakeResult{Output: ""})
		rec.On("show-options", fakeResult{Output: "/usr/bin/fish"})

		runner, err := runnerClient().NewCommandRunner(time.Minute)
		assert.NoError(t, err)
		assert.Equal(t, "/tmp/tmux-1000/default", runner.socket)
		assert.Equal(t, shellFish, runner.shell)
		assert.Equal(t, time.Minute, runner.timeout)
		assert.Equal(t, []string{"display-message", "-p", "#{socket_path}"}, rec.ArgsFor("display-message"))
	})

	t.Run("prefers default-command to default-shell", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})
		rec.On("show-options", fakeResult{Output: "exec fish"})

		runner, err := runnerClient().NewCommandRunner(0)
		assert.NoError(t, err)
		assert.Equal(t, shellFish, runner.shell)
		assert.Equal(t, []string{"show-options", "-gqv", "default-command"}, rec.ArgsFor("show-options"))
	})

	t.Run("warns about a shell that it does not recognise", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})
		rec.On("show-options", fakeResult{Output: ""})
		rec.On("show-options", fakeResult{Output: "/usr/bin/nu"})

		var logs bytes.Buffer
		client := runnerClient()
		client.logger = slog.New(slog.NewTextHandler(&logs, nil))

		runner, err := client.NewCommandRunner(0)
		assert.NoError(t, err)
		assert.Equal(t, shellPOSIX, runner.shell)
		assert.Contains(t, logs.String(), "does not recognise this shell")
		assert.Contains(t, logs.String(), "shell=/usr/bin/nu")
	})

	t.Run("errors when the socket path cannot be read", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Err: errors.New("no server")})

		_, err := runnerClient().NewCommandRunner(0)
		assert.ErrorContains(t, err, "socket path")
		assert.False(t, rec.Called("show-options"))
	})

	t.Run("errors when an option cannot be read", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})
		rec.On("show-options", fakeResult{Err: errors.New("no server")})

		_, err := runnerClient().NewCommandRunner(0)
		assert.ErrorContains(t, err, "default-command")
	})
}

func TestCommandRunnerScript(t *testing.T) {
	runner := testRunner(0)
	tmux := `'/usr/bin/tmux' -S '/tmp/tmux-1000/default'`

	t.Run("signals before the last command", func(t *testing.T) {
		assert.Equal(t,
			tmux+" delete-buffer -b glaze-x\ncommand eval ' npm i &'\ncommand eval ' echo a # note'\n"+tmux+" wait-for -S glaze-x\ncommand eval ' nvim'\n",
			runner.script("glaze-x", []string{"npm i &", "echo a # note", "nvim"}),
		)
	})

	t.Run("does not signal for a single command", func(t *testing.T) {
		assert.Equal(t, tmux+" delete-buffer -b glaze-x\ncommand eval ' nvim'\n", runner.script("glaze-x", []string{"nvim"}))
	})

	t.Run("quotes each command for the shell", func(t *testing.T) {
		script := runner.script("glaze-x", []string{`echo "it's" \`, "-x"})
		assert.Contains(t, script, "\ncommand eval ' echo \"it'\\''s\" \\'\n")
		assert.Contains(t, script, "\ncommand eval ' -x'\n")
	})

	t.Run("uses plain eval in zsh", func(t *testing.T) {
		zsh := testRunner(0)
		zsh.shell = shellZsh
		assert.Contains(t, zsh.script("glaze-x", []string{"nvim"}), "\neval ' nvim'\n")
	})

	t.Run("quotes each command for fish", func(t *testing.T) {
		fish := testRunner(0)
		fish.shell = shellFish
		assert.Contains(t, fish.script("glaze-x", []string{`echo it's \`}), "\neval ' echo it\\'s \\\\'\n")
	})
}

func TestCommandRunnerEvalLine(t *testing.T) {
	t.Run("posix", func(t *testing.T) {
		assert.Equal(t,
			` eval "$('/usr/bin/tmux' -S '/tmp/tmux-1000/default' show-buffer -b glaze-x)"`,
			testRunner(0).evalLine("glaze-x"),
		)
	})

	t.Run("fish", func(t *testing.T) {
		runner := testRunner(0)
		runner.shell = shellFish
		assert.Equal(t,
			` eval ('/usr/bin/tmux' -S '/tmp/tmux-1000/default' show-buffer -b glaze-x | string collect)`,
			runner.evalLine("glaze-x"),
		)
	})
}

func TestCommandRunnerQuote(t *testing.T) {
	cases := []struct {
		name, in, posix, fish string
	}{
		{"plain", "/tmp/tmux-1000/default", `'/tmp/tmux-1000/default'`, `'/tmp/tmux-1000/default'`},
		{"single quote", "/tmp/it's", `'/tmp/it'\''s'`, `'/tmp/it\'s'`},
		{"backslash", `/tmp/a\b`, `'/tmp/a\b'`, `'/tmp/a\\b'`},
		{"shell syntax", "/tmp/$(id);`x` \"y\"", "'/tmp/$(id);`x` \"y\"'", "'/tmp/$(id);`x` \"y\"'"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runner := testRunner(0)
			assert.Equal(t, c.posix, runner.quote(c.in))

			runner.shell = shellFish
			assert.Equal(t, c.fish, runner.quote(c.in))
		})
	}
}

func TestCommandRunnerRun(t *testing.T) {
	t.Run("does nothing without commands", func(t *testing.T) {
		rec := setupRecorder(t)

		assert.NoError(t, testRunner(0).Run("%1", nil))
		assert.Empty(t, rec.Calls)
	})

	t.Run("loads, types and waits", func(t *testing.T) {
		rec := setupRecorder(t)

		assert.NoError(t, testRunner(0).Run("%1", []string{"-x !y", "C-c"}))

		load := rec.ArgsFor("load-buffer")
		assert.Len(t, load, 4)
		name := load[2]
		assert.Regexp(t, `^glaze-[0-9a-f]{16}$`, name)
		assert.Equal(t, []string{"load-buffer", "-b", name, "-"}, load)

		input := rec.InputFor("load-buffer")
		assert.True(t, strings.HasSuffix(input, "\ncommand eval ' -x !y'\n'/usr/bin/tmux' -S '/tmp/tmux-1000/default' wait-for -S "+name+"\ncommand eval ' C-c'\n"))

		var sends [][]string
		for _, call := range rec.Calls {
			if call[0] == "send-keys" {
				sends = append(sends, call)
			}
		}

		assert.Equal(t, [][]string{
			{"send-keys", "-l", "-t", "%1", "--", testRunner(0).evalLine(name)},
			{"send-keys", "-t", "%1", "Enter"},
		}, sends)

		assert.Equal(t, []string{"wait-for", name}, rec.ArgsFor("wait-for"))
	})

	t.Run("does not wait for a single command", func(t *testing.T) {
		rec := setupRecorder(t)

		assert.NoError(t, testRunner(0).Run("%1", []string{"nvim"}))
		assert.False(t, rec.Called("wait-for"))
	})

	t.Run("uses a new buffer name for each run", func(t *testing.T) {
		rec := setupRecorder(t)

		runner := testRunner(0)
		assert.NoError(t, runner.Run("%1", []string{"nvim"}))
		assert.NoError(t, runner.Run("%2", []string{"nvim"}))

		var names []string
		for _, call := range rec.Calls {
			if call[0] == "load-buffer" {
				names = append(names, call[2])
			}
		}

		assert.Len(t, names, 2)
		assert.NotEqual(t, names[0], names[1])
	})

	t.Run("errors when the buffer cannot be loaded", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("load-buffer", fakeResult{Err: errors.New("no server")})

		err := testRunner(0).Run("%1", []string{"echo a", "nvim"})
		assert.ErrorContains(t, err, "could not load the commands")
		assert.False(t, rec.Called("send-keys"))
	})

	t.Run("deletes the buffer when the line cannot be typed", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send-keys", fakeResult{Err: errors.New("can't find pane")})

		err := testRunner(0).Run("%1", []string{"echo a", "nvim"})
		assert.ErrorContains(t, err, "could not type the command line into pane `%1`")
		assert.Equal(t, rec.ArgsFor("load-buffer")[2], rec.ArgsFor("delete-buffer")[2])
		assert.False(t, rec.Called("wait-for"))
	})

	t.Run("deletes the buffer when Enter cannot be sent", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send-keys", fakeResult{})
		rec.On("send-keys", fakeResult{Err: errors.New("can't find pane")})

		assert.Error(t, testRunner(0).Run("%1", []string{"echo a", "nvim"}))
		assert.True(t, rec.Called("delete-buffer"))
	})

	t.Run("propagates wait errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("wait-for", fakeResult{Err: errors.New("server exited")})

		err := testRunner(0).Run("%1", []string{"echo a", "nvim"})
		assert.ErrorContains(t, err, "server exited")
		assert.NotErrorIs(t, err, ErrPaneExited)
	})
}

func TestCommandRunnerWait(t *testing.T) {
	// blockWait queues a wait that returns only when the runner signals the channel itself.
	blockWait := func(rec *CommandRecorder) {
		release := make(chan struct{})
		rec.On("wait-for", fakeResult{OnExec: func() { <-release }})
		rec.On("wait-for", fakeResult{OnExec: func() { close(release) }})
	}

	t.Run("stops when the pane is gone", func(t *testing.T) {
		fastPaneChecks(t)
		rec := setupRecorder(t)
		blockWait(rec)
		rec.On("display-message", fakeResult{Output: ""})

		assert.ErrorIs(t, waitFor(t, testRunner(0)), ErrPaneExited)
		assert.Equal(t, []string{"display-message", "-p", "-t", "%1", "#{pane_id} #{pane_dead}"}, rec.ArgsFor("display-message"))
		assert.Equal(t, []string{"wait-for", "-S", "glaze-x"}, rec.Calls[len(rec.Calls)-2])
		assert.Equal(t, []string{"delete-buffer", "-b", "glaze-x"}, rec.Calls[len(rec.Calls)-1])
	})

	t.Run("stops when the pane is dead", func(t *testing.T) {
		fastPaneChecks(t)
		rec := setupRecorder(t)
		blockWait(rec)
		rec.On("display-message", fakeResult{Output: "%1 1"})

		assert.ErrorIs(t, waitFor(t, testRunner(0)), ErrPaneExited)
	})

	t.Run("stops when the pane cannot be checked", func(t *testing.T) {
		fastPaneChecks(t)
		rec := setupRecorder(t)
		blockWait(rec)
		rec.On("display-message", fakeResult{Err: errors.New("no server")})

		assert.ErrorIs(t, waitFor(t, testRunner(0)), ErrPaneExited)
	})

	t.Run("accepts a signal that arrives just before the shell exits", func(t *testing.T) {
		fastPaneChecks(t)
		rec := setupRecorder(t)
		release := make(chan struct{})
		rec.On("wait-for", fakeResult{OnExec: func() { <-release }})
		rec.On("display-message", fakeResult{Output: "", OnExec: func() { close(release) }})

		assert.NoError(t, waitFor(t, testRunner(0)))
		assert.False(t, rec.Called("delete-buffer"))
	})

	t.Run("stops when the wait fails just after the pane dies", func(t *testing.T) {
		fastPaneChecks(t)
		rec := setupRecorder(t)
		release := make(chan struct{})
		rec.On("wait-for", fakeResult{Err: errors.New("server exited"), OnExec: func() { <-release }})
		rec.On("display-message", fakeResult{Output: "", OnExec: func() { close(release) }})

		assert.ErrorIs(t, waitFor(t, testRunner(0)), ErrPaneExited)
		assert.True(t, rec.Called("delete-buffer"))
	})

	t.Run("keeps waiting while the pane is alive", func(t *testing.T) {
		fastPaneChecks(t)
		rec := setupRecorder(t)
		release := make(chan struct{})
		rec.On("wait-for", fakeResult{OnExec: func() { <-release }})
		rec.On("display-message", fakeResult{Output: "%1 0"})
		rec.On("display-message", fakeResult{Output: "%1 0"})
		rec.On("display-message", fakeResult{Output: "%1 0", OnExec: func() { close(release) }})

		assert.NoError(t, waitFor(t, testRunner(0)))
	})

	t.Run("stops when the run is cancelled and removes the buffer", func(t *testing.T) {
		rec := setupRecorder(t)
		release := make(chan struct{})
		rec.On("wait-for", fakeResult{OnExec: func() { <-release }})
		t.Cleanup(func() { close(release) })

		cause := errors.New("stopped on SIGTERM")
		ctx, cancel := context.WithCancelCause(context.Background())
		runner := testRunner(0)
		runner.client = runner.client.WithContext(ctx)
		time.AfterFunc(10*time.Millisecond, func() { cancel(cause) })

		assert.ErrorIs(t, waitFor(t, runner), cause)
		assert.Equal(t, []string{"delete-buffer", "-b", "glaze-x"}, rec.ArgsFor("delete-buffer"))
	})

	t.Run("stops after the timeout", func(t *testing.T) {
		rec := setupRecorder(t)
		blockWait(rec)

		assert.ErrorIs(t, waitFor(t, testRunner(10*time.Millisecond)), ErrCommandTimeout)
		assert.True(t, rec.Called("delete-buffer"))
	})
}
