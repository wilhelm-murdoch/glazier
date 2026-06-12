package actions

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

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
		rec.On("list-sessions", tmuxtest.Result{Status: 0})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp\n$2;gig-watson;/home/v"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1\n@2;2;logs;tiled;0"})
		rec.On("lsw", tmuxtest.Result{Output: "@3;1;main;tiled;1"})

		assert.NoError(t, ls.Run())

		rendered := out.String()
		assert.Contains(t, rendered, "NAME")
		assert.Contains(t, rendered, "demo")
		assert.Contains(t, rendered, "gig-watson")
		assert.Contains(t, rendered, "/home/v")
		// demo has two windows, gig-watson has one.
		assert.Regexp(t, `demo\s+2\s+/tmp`, rendered)
		assert.Regexp(t, `gig-watson\s+1\s+/home/v`, rendered)
	})

	t.Run("marks the attached session when run inside tmux", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-501/default,1234,0")
		ls, rec, out := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{Status: 0})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("display-message", tmuxtest.Result{Output: "demo"})
		rec.On("lsw", tmuxtest.Result{Output: "@1;1;main;tiled;1"})

		assert.NoError(t, ls.Run())

		assert.Regexp(t, `demo\*\s+1\s+/tmp`, out.String())
	})

	t.Run("errors when no tmux server is running", func(t *testing.T) {
		ls, rec, _ := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{Status: 1})

		err := ls.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no running tmux server")
	})

	t.Run("propagates window listing failures", func(t *testing.T) {
		t.Setenv("TMUX", "")
		ls, rec, _ := buildLs(t)
		rec.On("list-sessions", tmuxtest.Result{Status: 0})
		rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
		rec.On("lsw", tmuxtest.Result{Err: assert.AnError})

		err := ls.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not list windows")
	})
}
