package actions

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// captureSetup holds a client on a recorder, the session that one `ls` line describes and a logger that records every level.
type captureSetup struct {
	client  tmux.Client
	session *tmux.Session
	rec     *tmuxtest.Recorder
	log     *logger.Logger
	logs    *bytes.Buffer
}

// newCapture installs a recorder and builds the client and the session of sessionLine; it skips the test when tmux is not installed.
func newCapture(t *testing.T, sessionLine string) captureSetup {
	t.Helper()

	rec := tmuxtest.New().Default(tmuxtest.Result{})
	rec.Install(t)

	client, err := tmux.NewClient("", "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if errors.Is(err, tmux.ErrUnreachable) {
		t.Skipf("could not build a tmux client (tmux likely unavailable): %v", err)
	}

	require.NoError(t, err)

	session, err := client.NewSessionFromLine(sessionLine)
	require.NoError(t, err)

	var logs bytes.Buffer
	log := &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))}

	return captureSetup{client: client, session: session, rec: rec, log: log, logs: &logs}
}

// subcommands returns the first word of every recorded tmux call, in order.
func subcommands(rec *tmuxtest.Recorder) []string {
	var subs []string
	for _, call := range rec.Calls {
		subs = append(subs, call[0])
	}

	return subs
}

func TestCaptureSession(t *testing.T) {
	t.Run("keeps what tmux reports and drops the names that tmux chose", func(t *testing.T) {
		dir := t.TempDir()
		gone := filepath.Join(dir, "gone")
		c := newCapture(t, "$1;demo;"+dir)

		c.rec.On("display-message", tmuxtest.Result{Output: "buildhost"}) // #{host}
		c.rec.On("lsw", tmuxtest.Result{Output: "@1;1;zsh;bb62,80x24,0,0;1\n@2;2;logs;bb62,80x24,0,0;0"})
		c.rec.On("display-message", tmuxtest.Result{Output: "1"}) // @1 has automatic-rename on
		c.rec.On("lsp", tmuxtest.Result{Output: "%1;1;buildhost;1;" + gone})
		c.rec.On("display-message", tmuxtest.Result{Output: "0"}) // @2 was named
		c.rec.On("lsp", tmuxtest.Result{Output: "%2;1;tail;1;" + dir + "\n%3;2;buildhost;0;"})

		captured, err := captureSession(c.client, c.session, c.log)
		require.NoError(t, err)

		assert.Equal(t, "demo", captured.Name)
		assert.Equal(t, dir, captured.StartingDirectory)
		assert.False(t, captured.DirectoryMissing)
		require.Len(t, captured.Windows, 2)

		// The window that tmux named keeps its live name and loses the profile name; the layout and the focus stay.
		zsh := captured.Windows[0]
		assert.Empty(t, zsh.Name)
		assert.Equal(t, "zsh", zsh.LiveName)
		assert.True(t, zsh.Automatic)
		assert.Equal(t, "bb62,80x24,0,0", zsh.Layout)
		assert.True(t, zsh.Focus)
		require.Len(t, zsh.Panes, 1)

		// A pane with the host title keeps the title as a fact and loses the profile name; a gone directory is flagged, not blanked.
		host := zsh.Panes[0]
		assert.Empty(t, host.Name)
		assert.Equal(t, "buildhost", host.Title)
		assert.True(t, host.HostTitle)
		assert.Equal(t, gone, host.StartingDirectory)
		assert.True(t, host.DirectoryMissing)
		assert.False(t, host.DirectoryUnknown)
		assert.True(t, host.Focus)

		logs := captured.Windows[1]
		assert.Equal(t, "logs", logs.Name)
		assert.Equal(t, "logs", logs.LiveName)
		assert.False(t, logs.Automatic)
		assert.False(t, logs.Focus)
		require.Len(t, logs.Panes, 2)

		tail := logs.Panes[0]
		assert.Equal(t, "tail", tail.Name)
		assert.Equal(t, "tail", tail.Title)
		assert.False(t, tail.HostTitle)
		assert.Equal(t, dir, tail.StartingDirectory)
		assert.False(t, tail.DirectoryMissing)
		assert.False(t, tail.DirectoryUnknown)
		assert.True(t, tail.Focus)

		// An empty path means that tmux reported none, which is unknown and not missing.
		unknown := logs.Panes[1]
		assert.True(t, unknown.HostTitle)
		assert.Empty(t, unknown.StartingDirectory)
		assert.True(t, unknown.DirectoryUnknown)
		assert.False(t, unknown.DirectoryMissing)
		assert.False(t, unknown.Focus)

		// The capture only reads: the host once, then the windows, then per window the option and the panes.
		assert.Equal(t, []string{"display-message", "lsw", "display-message", "lsp", "display-message", "lsp"}, subcommands(c.rec))
		assert.Contains(t, c.rec.Calls[0], "#{host}")
		assert.Contains(t, c.rec.Calls[2], "#{automatic-rename}")

		// The capture lines are debug, so a plain save or diff run prints them no more.
		assert.Equal(t, 2, strings.Count(c.logs.String(), `level=DEBUG msg="capturing window"`), c.logs.String())
		assert.Equal(t, 3, strings.Count(c.logs.String(), `level=DEBUG msg="capturing pane"`), c.logs.String())
		assert.NotContains(t, c.logs.String(), "level=INFO")
		assert.NotContains(t, c.logs.String(), "level=WARN")
	})

	t.Run("flags a session directory that no longer exists and keeps the path", func(t *testing.T) {
		gone := filepath.Join(t.TempDir(), "gone")
		c := newCapture(t, "$1;demo;"+gone)
		c.rec.On("display-message", tmuxtest.Result{Output: "buildhost"})
		c.rec.On("lsw", tmuxtest.Result{})

		captured, err := captureSession(c.client, c.session, c.log)
		require.NoError(t, err)

		assert.Equal(t, gone, captured.StartingDirectory)
		assert.True(t, captured.DirectoryMissing)
		assert.Empty(t, captured.Windows)
	})

	t.Run("reads a file in place of a directory as missing", func(t *testing.T) {
		dir := t.TempDir()
		c := newCapture(t, "$1;demo;"+dir)
		c.rec.On("display-message", tmuxtest.Result{Output: "buildhost"})
		c.rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1"})
		c.rec.On("display-message", tmuxtest.Result{Output: "0"})
		c.rec.On("lsp", tmuxtest.Result{Output: "%1;1;shell;1;" + filepath.Join(dir, ".glaze")})

		captured, err := captureSession(c.client, c.session, c.log)
		require.NoError(t, err)

		assert.True(t, captured.Windows[0].Panes[0].DirectoryMissing)
	})

	t.Run("fails when the host name cannot be read", func(t *testing.T) {
		c := newCapture(t, "$1;demo;/tmp")
		c.rec.On("display-message", tmuxtest.Failure("can't find session: $1"))

		_, err := captureSession(c.client, c.session, c.log)
		assert.ErrorContains(t, err, "can't find session")
		assert.False(t, c.rec.Called("lsw"))
	})

	t.Run("says that the session changed when a window is gone before its panes are read", func(t *testing.T) {
		c := newCapture(t, "$1;demo;/tmp")
		c.rec.On("display-message", tmuxtest.Result{Output: "buildhost"})
		c.rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1\n@2;2;extra;tiled;0"})
		c.rec.On("display-message", tmuxtest.Result{Output: "0"})
		c.rec.On("lsp", tmuxtest.Result{Output: "%1;1;shell;1;/tmp"})
		c.rec.On("display-message", tmuxtest.Result{Output: "0"})
		c.rec.On("lsp", tmuxtest.Failure("can't find window: @2"))

		_, err := captureSession(c.client, c.session, c.log)
		assert.ErrorContains(t, err, "the session changed while glaze read it; run the command again")
		assert.ErrorContains(t, err, "can't find window: @2")
	})

	t.Run("says that the session changed when a window is gone before its option is read", func(t *testing.T) {
		c := newCapture(t, "$1;demo;/tmp")
		c.rec.On("display-message", tmuxtest.Result{Output: "buildhost"})
		c.rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1"})
		c.rec.On("display-message", tmuxtest.Failure("can't find window: @1"))

		_, err := captureSession(c.client, c.session, c.log)
		assert.ErrorContains(t, err, "the session changed while glaze read it; run the command again")
		assert.False(t, c.rec.Called("lsp"))
	})

	t.Run("fails on a window line it cannot parse", func(t *testing.T) {
		c := newCapture(t, "$1;demo;/tmp")
		c.rec.On("display-message", tmuxtest.Result{Output: "buildhost"})
		c.rec.On("lsw", tmuxtest.Result{Output: "not a window line"})

		_, err := captureSession(c.client, c.session, c.log)
		assert.Error(t, err)
	})
}

func TestPruneMissingDirectories(t *testing.T) {
	var logs bytes.Buffer
	log := &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

	captured := savedSession{
		Name:              "demo",
		StartingDirectory: "/gone",
		DirectoryMissing:  true,
		Windows: []savedWindow{
			{Name: "main", Panes: []savedPane{
				{Name: "shell", Title: "shell", StartingDirectory: "/gone/shell", DirectoryMissing: true},
				{Name: "tail", Title: "tail", StartingDirectory: "/srv"},
				{Title: "buildhost", HostTitle: true, DirectoryUnknown: true},
			}},
		},
	}

	pruneMissingDirectories(&captured, log)

	// Each missing directory is blanked with a warning that names the directory; the others stay as they were.
	assert.Empty(t, captured.StartingDirectory)
	assert.Empty(t, captured.Windows[0].Panes[0].StartingDirectory)
	assert.Equal(t, "/srv", captured.Windows[0].Panes[1].StartingDirectory)
	assert.Empty(t, captured.Windows[0].Panes[2].StartingDirectory)

	assert.Equal(t, 2, strings.Count(logs.String(), "the directory no longer exists, so the profile leaves it out"), logs.String())
	assert.Contains(t, logs.String(), "session=demo directory=/gone\n")
	assert.Contains(t, logs.String(), "pane=shell directory=/gone/shell\n")
	assert.NotContains(t, logs.String(), "/srv")

	// The flags stay, so a caller can still tell a pruned directory from one that tmux never reported.
	assert.True(t, captured.DirectoryMissing)
	assert.True(t, captured.Windows[0].Panes[0].DirectoryMissing)
	assert.True(t, captured.Windows[0].Panes[2].DirectoryUnknown)
}
