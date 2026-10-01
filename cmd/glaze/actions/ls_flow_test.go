package actions

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// buildLs constructs a fully wired ActionLs with a tmuxtest recorder installed
// and its output captured into the returned buffer.
func buildLs(t *testing.T) (*ActionLs, *tmuxtest.Recorder, *bytes.Buffer) {
	t.Helper()

	rec := tmuxtest.New().Default(tmuxtest.Result{})
	rec.Install(t)

	var (
		action *ActionLs
		actErr error
	)

	cmd := &cli.Command{
		Name: "ls",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "socket-path"},
			&cli.StringFlag{Name: "socket-name"},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			action, actErr = NewLs(c, "critical")
			return nil
		},
	}

	assert.NoError(t, cmd.Run(context.Background(), []string{"ls"}))
	if actErr != nil {
		t.Skipf("could not construct ActionLs (tmux likely unavailable): %v", actErr)
	}

	out := &bytes.Buffer{}
	action.out = out

	return action, rec, out
}

func TestActionLsRun(t *testing.T) {
	t.Run("renders a table of sessions with window counts", func(t *testing.T) {
		t.Setenv("TMUX", "")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp\n$2;gig-watson;/home/v"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1\n@2;2;logs;tiled;0"})
		rec.On("lsw", tmuxtest.Result{Output: "@3;1;main;tiled;1"})

		assert.NoError(t, ls.Run(context.Background()))

		rendered := out.String()
		assert.Contains(t, rendered, "NAME")
		assert.Contains(t, rendered, "demo")
		assert.Contains(t, rendered, "gig-watson")
		assert.Contains(t, rendered, "/home/v")
		// demo has two windows, gig-watson has one.
		assert.Regexp(t, `demo\s+2\s+/tmp`, rendered)
		assert.Regexp(t, `gig-watson\s+1\s+/home/v`, rendered)
	})

	t.Run("marks the session of the pane that glaze runs in", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
		t.Setenv("TMUX_PANE", "%4")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp\n$2;other;/srv"})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Output: "$2;other;/srv"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1"})
		rec.On("lsw", tmuxtest.Result{Output: "@2;1;main;tiled;1"})

		assert.NoError(t, ls.Run(context.Background()))

		assert.Regexp(t, `other\*\s+1\s+/srv`, out.String())
		assert.NotRegexp(t, `demo\*`, out.String())
	})

	t.Run("prints nothing and succeeds when no tmux server is running", func(t *testing.T) {
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Failure("no server running on /tmp/tmux-1000/default"))

		var logs bytes.Buffer
		ls.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		assert.NoError(t, ls.Run(context.Background()))
		assert.Empty(t, out.String())
		assert.Contains(t, logs.String(), "no tmux server is running")
		assert.False(t, rec.Called("ls"))
	})

	t.Run("errors when tmux is unreachable", func(t *testing.T) {
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Failure("error connecting to /tmp/tmux-0/default (Permission denied)"))

		assert.ErrorIs(t, ls.Run(context.Background()), tmux.ErrUnreachable)
		assert.Empty(t, out.String())
	})

	t.Run("propagates window listing failures", func(t *testing.T) {
		t.Setenv("TMUX", "")
		ls, rec, _ := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("lsw", tmuxtest.Result{Err: assert.AnError})

		err := ls.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not list windows")
	})

	t.Run("marks no session inside another tmux server", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
		t.Setenv("TMUX_PANE", "%4")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp\n$2;other;/srv"})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/work"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1"})
		rec.On("lsw", tmuxtest.Result{Output: "@2;1;main;tiled;1"})

		assert.NoError(t, ls.Run(context.Background()))
		assert.NotContains(t, out.String(), "*")
		assert.Equal(t, 1, rec.CountOf("display-message"))
	})

	t.Run("errors when the current session cannot be determined", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
		t.Setenv("TMUX_PANE", "%4")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-501/default"})
		rec.On("display-message", tmuxtest.Result{Err: assert.AnError})

		err := ls.Run(context.Background())
		assert.ErrorIs(t, err, assert.AnError)
		assert.Empty(t, out.String())
	})
}
