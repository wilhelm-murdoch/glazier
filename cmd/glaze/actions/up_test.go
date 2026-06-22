package actions

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// newTestUp builds an ActionUp wired to a fake-command-backed tmux client and a
// session named "demo". The recorder routes canned tmux output per subcommand;
// the default result returns two space-separated tokens so base-index lookups
// (`tmux show ...`) parse, and is harmless for commands that ignore output.
func newTestUp(t *testing.T) (*ActionUp, *tmuxtest.Recorder) {
	t.Helper()

	log := logger.New(logger.LevelCritical)

	client, err := tmux.NewClient("", "", log.Logger)
	if err != nil {
		t.Skipf("tmux binary not available: %v", err)
	}

	rec := tmuxtest.New().Default(tmuxtest.Result{Output: "base-index 1"})
	rec.Install(t)

	rec.On("ls", tmuxtest.Result{Output: "$1;demo;/tmp"})
	session, err := client.FindSessionByName("demo")
	assert.NoError(t, err)

	return &ActionUp{
		ActionBase: ActionBase{Logger: log},
		tmux:       client,
		session:    session,
	}, rec
}

func windowWithPane(name string, layout enums.Layout, pane *decoders.Pane) *decoders.Window {
	window := &decoders.Window{
		Base:   &decoders.Base{Name: name},
		Layout: layout,
	}
	window.Panes = []*decoders.Pane{pane}
	return window
}

func TestActionUpApplySessionSettings(t *testing.T) {
	t.Run("applies envs, hooks and options", func(t *testing.T) {
		up, rec := newTestUp(t)

		profile := &decoders.Session{
			Base: &decoders.Base{
				Name:    "demo",
				Hooks:   map[string]string{"session-created": "echo hi"},
				Options: map[string]string{"base-index": "1"},
			},
			Envs: map[string]string{"EDITOR": "vim"},
		}

		assert.NoError(t, up.applySessionSettings(profile))

		assert.True(t, rec.Called("setenv"))
		assert.True(t, rec.Called("set-hook"))
		assert.True(t, rec.Called("set-option"))

		assert.Contains(t, rec.ArgsFor("setenv"), "EDITOR")
		assert.Contains(t, rec.ArgsFor("set-hook"), "session-created")
		assert.Contains(t, rec.ArgsFor("set-option"), "base-index")
	})

	t.Run("is a no-op when the session is nil", func(t *testing.T) {
		up, rec := newTestUp(t)
		up.session = nil

		profile := &decoders.Session{
			Envs: map[string]string{"EDITOR": "vim"},
		}

		assert.NoError(t, up.applySessionSettings(profile))
		assert.False(t, rec.Called("setenv"))
	})

	t.Run("propagates errors from tmux", func(t *testing.T) {
		up, rec := newTestUp(t)
		rec.On("setenv", tmuxtest.Result{Err: errors.New("boom")})

		profile := &decoders.Session{
			Base: &decoders.Base{
				Name: "demo",
			},
			Envs: map[string]string{"EDITOR": "vim"},
		}

		err := up.applySessionSettings(profile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not set env")
	})
}

func TestActionUpGenerateWindows(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;ice-breaker;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;breach;1"})

	pane := &decoders.Pane{
		Base:     &decoders.Base{Name: "breach", StartingDirectory: "/tmp"},
		Commands: []string{"cd /tmp", "htop"},
	}
	window := windowWithPane("ice-breaker", enums.LayoutTiled, pane)

	assert.NoError(t, up.generateWindows([]*decoders.Window{window}))

	// Window + pane were created, the default pane killed, layout selected.
	assert.True(t, rec.Called("neww"))
	assert.True(t, rec.Called("splitw"))
	assert.True(t, rec.Called("killp"))
	assert.True(t, rec.Called("selectl"))

	// Two commands: the first is serialised with wait-for, the final command
	// is sent fire-and-forget so a long-running command cannot hang.
	assert.Equal(t, 2, rec.CountOf("send"))
	assert.Equal(t, 1, rec.CountOf("wait-for"))

	var sends [][]string
	for _, c := range rec.Calls {
		if len(c) > 0 && c[0] == "send" {
			sends = append(sends, c)
		}
	}
	assert.Contains(t, sends[0][len(sends[0])-2], "cd /tmp ; tmux wait-for -S")
	assert.Contains(t, sends[1][len(sends[1])-2], "htop")
	assert.NotContains(t, sends[1][len(sends[1])-2], "wait-for")
}

func TestActionUpProvisionSessionRunsSessionCommands(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1"})
	rec.On("lsw", tmuxtest.Result{Output: "@1;1;default;tiled;1"})

	pane := &decoders.Pane{Base: &decoders.Base{Name: "p"}, Commands: []string{"echo pane"}}
	window := windowWithPane("w", enums.LayoutTiled, pane)

	profile := &decoders.Session{
		Base:     &decoders.Base{Name: "demo"},
		Commands: []string{"echo session"},
	}
	profile.Windows = []*decoders.Window{window}

	assert.NoError(t, up.provisionSession(profile))

	// The default window tmux created was removed.
	assert.True(t, rec.Called("killw"))

	// Each command list has a single, final command, so both are sent
	// fire-and-forget with no wait-for synchronisation.
	assert.Equal(t, 2, rec.CountOf("send"))
	assert.Equal(t, 0, rec.CountOf("wait-for"))
}

func TestActionUpProvisionSessionSerialisesAllButLastSessionCommand(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1"})
	rec.On("lsw", tmuxtest.Result{Output: "@1;1;default;tiled;1"})

	pane := &decoders.Pane{Base: &decoders.Base{Name: "p"}}
	window := windowWithPane("w", enums.LayoutTiled, pane)

	profile := &decoders.Session{
		Base:     &decoders.Base{Name: "demo"},
		Commands: []string{"nvm use 18", "tail -f log"},
	}
	profile.Windows = []*decoders.Window{window}

	assert.NoError(t, up.provisionSession(profile))

	// First session command waits, the final long-running command does not.
	assert.Equal(t, 2, rec.CountOf("send"))
	assert.Equal(t, 1, rec.CountOf("wait-for"))
}
