package actions

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/tmuxtest"
)

// newTestUp builds an ActionUp for the session "demo" whose tmux commands go to a recorder.
func newTestUp(t *testing.T) (*ActionUp, *tmuxtest.Recorder) {
	t.Helper()

	log := logger.New(logger.LevelCritical)

	client, err := tmux.NewClient("", "", log.Logger)
	if err != nil {
		t.Skipf("tmux binary not available: %v", err)
	}

	rec := tmuxtest.New().Default(tmuxtest.Result{})
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
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;breach;1;/tmp"})

	pane := &decoders.Pane{
		Base:     &decoders.Base{Name: "breach", StartingDirectory: "/tmp"},
		Commands: []string{"cd /tmp", "htop"},
	}
	window := windowWithPane("ice-breaker", enums.LayoutTiled, pane)

	assert.NoError(t, up.generateWindows([]*decoders.Window{window}, nil))

	// Window + pane were created, the default pane killed, layout selected.
	assert.True(t, rec.Called("neww"))
	assert.True(t, rec.Called("splitw"))
	assert.True(t, rec.Called("killp"))
	assert.True(t, rec.Called("selectl"))

	// Both commands go into one buffer, and glaze waits for every command but the last.
	assert.Equal(t, 1, rec.CountOf("load-buffer"))
	assert.Equal(t, 1, rec.CountOf("wait-for"))
	assert.Equal(t, "%2", rec.ArgsFor("send-keys")[3])

	input := rec.InputsFor("load-buffer")[0]
	assert.Regexp(t, `\ncommand eval ' cd /tmp'\n.* wait-for -S glaze-[0-9a-f]+\ncommand eval ' htop'\n$`, input)
}

func TestActionUpWarnsAboutRenamedWindowAndPane(t *testing.T) {
	up, rec := newTestUp(t)

	var logs bytes.Buffer
	up.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w-z;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;p-z;1;/tmp"})

	window := windowWithPane(`w\z`, enums.LayoutTiled, &decoders.Pane{Base: &decoders.Base{Name: "p\tz"}})
	assert.NoError(t, up.generateWindows([]*decoders.Window{window}, nil))

	assert.Contains(t, logs.String(), "this window name")
	assert.Contains(t, logs.String(), "tmux_name=w-z")
	assert.Contains(t, logs.String(), "this pane name")
	assert.Contains(t, logs.String(), "tmux_name=p-z")
}

func TestActionUpResizesPanesAfterTheLayout(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;a;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%3;2;b;1;/tmp"})

	window := windowWithPane("w", enums.LayoutEvenHorizontal, &decoders.Pane{
		Base:        &decoders.Base{Name: "a"},
		Size:        decoders.Size{X: "20"},
		Adjustments: []decoders.Adjustment{{Direction: enums.AdjustmentRight, Amount: "5"}},
	})
	window.Panes = append(window.Panes, &decoders.Pane{Base: &decoders.Base{Name: "b"}})

	assert.NoError(t, up.generateWindows([]*decoders.Window{window}, nil))

	// The declared layout comes first, then the size, then the adjust blocks.
	var order [][]string
	for _, call := range rec.Calls {
		if call[0] == "resizep" || (call[0] == "selectl" && call[len(call)-1] == "even-horizontal") {
			order = append(order, call)
		}
	}
	assert.Equal(t, [][]string{
		{"selectl", "-t", "@1", "even-horizontal"},
		{"resizep", "-t", "%2", "-x", "20"},
		{"resizep", "-t", "%2", "-R", "5"},
	}, order)
}

func TestActionUpIgnoresSizeInARawLayout(t *testing.T) {
	up, rec := newTestUp(t)

	var logs bytes.Buffer
	up.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;a;1;/tmp"})

	window := windowWithPane("w", enums.LayoutUnknown, &decoders.Pane{
		Base: &decoders.Base{Name: "a"},
		Size: decoders.Size{X: "20"},
	})
	window.LayoutRaw = "bb62,80x24,0,0"

	assert.NoError(t, up.generateWindows([]*decoders.Window{window}, nil))

	assert.False(t, rec.Called("resizep"))
	assert.Equal(t, "bb62,80x24,0,0", lastCall(rec, "selectl")[3])
	assert.Contains(t, logs.String(), "glaze ignores size and adjust")
	assert.Contains(t, logs.String(), "pane=a")
}

func TestActionUpGeneratePanesRebalancesAfterEachSplit(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;a;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%3;2;b;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%4;3;c;1;/tmp"})

	window := windowWithPane("w", enums.LayoutEvenHorizontal, &decoders.Pane{Base: &decoders.Base{Name: "a"}})
	window.Panes = append(window.Panes,
		&decoders.Pane{Base: &decoders.Base{Name: "b"}},
		&decoders.Pane{Base: &decoders.Base{Name: "c"}},
	)

	assert.NoError(t, up.generateWindows([]*decoders.Window{window}, nil))

	// Each split is followed by a tiled rebalance, and the declared layout comes last.
	var order []string
	for _, call := range rec.Calls {
		switch call[0] {
		case "splitw":
			order = append(order, "splitw")
		case "selectl":
			order = append(order, "selectl "+call[len(call)-1])
		}
	}
	assert.Equal(t, []string{
		"splitw", "selectl tiled",
		"splitw", "selectl tiled",
		"splitw", "selectl tiled",
		"selectl even-horizontal",
	}, order)
}

func TestActionUpGeneratePanesCreatesAllPanesFirst(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;runner;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%3;2;shell;1;/tmp"})

	window := windowWithPane("w", enums.LayoutTiled, &decoders.Pane{
		Base:     &decoders.Base{Name: "runner"},
		Commands: []string{"true; exit"},
	})
	window.Panes = append(window.Panes, &decoders.Pane{Base: &decoders.Base{Name: "shell"}})

	assert.NoError(t, up.generateWindows([]*decoders.Window{window}, nil))

	// A pane that exits at once must not be the parent of a later split.
	var order []string
	for _, call := range rec.Calls {
		if call[0] == "splitw" || call[0] == "load-buffer" {
			order = append(order, call[0])
		}
	}
	assert.Equal(t, []string{"splitw", "splitw", "load-buffer"}, order)
}

func TestActionUpProvisionSessionRunsSessionCommands(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1;/tmp"})
	rec.On("lsw", tmuxtest.Result{Output: "@1;1;default;tiled;1"})

	rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-1000/default"})
	rec.On("show-options", tmuxtest.Result{Output: ""})
	rec.On("show-options", tmuxtest.Result{Output: "/bin/bash"})
	rec.On("display-message", tmuxtest.Result{Output: "%9"})

	pane := &decoders.Pane{Base: &decoders.Base{Name: "p"}, Commands: []string{"pane"}}
	window := windowWithPane("w", enums.LayoutTiled, pane)

	profile := &decoders.Session{
		Base:     &decoders.Base{Name: "demo"},
		Commands: []string{"session"},
	}
	profile.Windows = []*decoders.Window{window}

	assert.NoError(t, up.provisionSession(profile))

	// The window tmux created is reused, not removed.
	assert.True(t, rec.Called("renamew"))
	assert.False(t, rec.Called("killw"))

	// Each list has one command, so glaze does not wait. The session command runs in the active pane.
	assert.Equal(t, []string{"command eval ' pane'", "command eval ' session'"}, commandsIn(rec))
	assert.Equal(t, 0, rec.CountOf("wait-for"))
	assert.Equal(t, []string{"send-keys", "-t", "%9", "Enter"}, lastCall(rec, "send-keys"))
}

func TestActionUpProvisionSessionUsesTheFirstWindow(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("lsw", tmuxtest.Result{Output: "@3;1;bash;tiled;1"})
	rec.On("neww", tmuxtest.Result{Output: "@4;2;second;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("lsp", tmuxtest.Result{Output: "%5;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%6;1;q;1;/tmp"})

	profile := &decoders.Session{Base: &decoders.Base{Name: "demo"}}
	profile.Windows = []*decoders.Window{
		windowWithPane("first", enums.LayoutTiled, &decoders.Pane{Base: &decoders.Base{Name: "p"}}),
		windowWithPane("second", enums.LayoutTiled, &decoders.Pane{Base: &decoders.Base{Name: "q"}}),
	}

	assert.NoError(t, up.provisionSession(profile))

	// Only the second window is created; the first reuses the window that tmux made.
	assert.Equal(t, []string{"renamew", "-t", "@3", "first"}, rec.ArgsFor("renamew"))
	assert.Equal(t, 1, rec.CountOf("neww"))
	assert.Subset(t, rec.ArgsFor("neww"), []string{"-n", "second"})
	assert.False(t, rec.Called("killw"))

	// Windows and panes are found by id, so provisioning never reads base-index.
	assert.Equal(t, 0, rec.CountOf("show"))
}

func TestActionUpProvisionSessionAppliesOptionsAtTheirScope(t *testing.T) {
	t.Run("a window option on the session applies to every window before its panes", func(t *testing.T) {
		up, rec := newTestUp(t)

		rec.On("show-options", tmuxtest.Result{Output: "history-limit 2000"})
		rec.On("show-options", tmuxtest.Result{Output: "remain-on-exit off"})
		rec.On("lsw", tmuxtest.Result{Output: "@3;1;bash;tiled;1"})
		rec.On("neww", tmuxtest.Result{Output: "@4;2;second;tiled;1"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
		rec.On("lsp", tmuxtest.Result{Output: "%5;1;default;1;/tmp"})
		rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1;/tmp"})
		rec.On("splitw", tmuxtest.Result{Output: "%6;1;q;1;/tmp"})

		profile := &decoders.Session{Base: &decoders.Base{
			Name:    "demo",
			Options: map[string]string{"remain-on-exit": "on", "history-limit": "5000"},
		}}
		profile.Windows = []*decoders.Window{
			windowWithPane("first", enums.LayoutTiled, &decoders.Pane{Base: &decoders.Base{Name: "p"}}),
			windowWithPane("second", enums.LayoutTiled, &decoders.Pane{Base: &decoders.Base{Name: "q"}}),
		}

		assert.NoError(t, up.provisionSession(profile))

		var events []string
		for _, call := range rec.Calls {
			switch {
			case call[0] == "set-option" && call[1] == "-w":
				events = append(events, "window "+call[3]+" "+call[4])
			case call[0] == "set-option":
				events = append(events, "session "+call[2]+" "+call[3])
			case call[0] == "splitw":
				events = append(events, "split")
			}
		}
		assert.Equal(t, []string{
			"session $1 history-limit",
			"window @3 remain-on-exit", "split",
			"window @4 remain-on-exit", "split",
		}, events)
	})

	t.Run("a session option on a window or pane warns and applies to the session", func(t *testing.T) {
		up, rec := newTestUp(t)

		var logs bytes.Buffer
		up.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

		rec.On("show-options", tmuxtest.Result{Output: "history-limit 2000\nstatus on"})
		rec.On("show-options", tmuxtest.Result{Output: "remain-on-exit off"})
		rec.On("lsw", tmuxtest.Result{Output: "@3;1;bash;tiled;1"})
		rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
		rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1;/tmp"})

		pane := &decoders.Pane{Base: &decoders.Base{Name: "p", Options: map[string]string{"status": "off"}}}
		window := windowWithPane("w", enums.LayoutTiled, pane)
		window.Options = map[string]string{"history-limit": "5000"}

		profile := &decoders.Session{Base: &decoders.Base{Name: "demo"}}
		profile.Windows = []*decoders.Window{window}

		assert.NoError(t, up.provisionSession(profile))

		var sessionOptions []string
		for _, call := range rec.Calls {
			if call[0] == "set-option" {
				assert.NotContains(t, call, "-w")
				assert.NotContains(t, call, "-p")
				sessionOptions = append(sessionOptions, call[3])
			}
		}
		assert.Equal(t, []string{"history-limit", "status"}, sessionOptions)
		assert.Contains(t, logs.String(), "declared_on=\"window `w`\"")
		assert.Contains(t, logs.String(), "declared_on=\"pane `p` in window `w`\"")
	})
}

func TestActionUpProvisionSessionSerialisesAllButLastSessionCommand(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1;/tmp"})
	rec.On("lsw", tmuxtest.Result{Output: "@1;1;default;tiled;1"})

	pane := &decoders.Pane{Base: &decoders.Base{Name: "p"}}
	window := windowWithPane("w", enums.LayoutTiled, pane)

	profile := &decoders.Session{
		Base:     &decoders.Base{Name: "demo"},
		Commands: []string{"nvm use 18", "tail -f log"},
	}
	profile.Windows = []*decoders.Window{window}

	assert.NoError(t, up.provisionSession(profile))

	// glaze waits for the first session command, but not for the final long-running command.
	assert.Equal(t, 1, rec.CountOf("load-buffer"))
	assert.Equal(t, 1, rec.CountOf("wait-for"))
}

func TestActionUpContinuesAfterCommandsTimeOut(t *testing.T) {
	up, rec := newTestUp(t)
	up.commandTimeout = 10 * time.Millisecond

	var logs bytes.Buffer
	up.Logger = &logger.Logger{Logger: slog.New(slog.NewTextHandler(&logs, nil))}

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;slow;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%3;2;next;1;/tmp"})
	rec.On("display-message", tmuxtest.Result{Output: "/tmp/tmux-1000/default"})
	rec.On("show-options", tmuxtest.Result{Output: "/bin/bash"})

	// The wait returns only when glaze gives up and signals the channel itself.
	release := make(chan struct{})
	rec.On("wait-for", tmuxtest.Result{OnExec: func() { <-release }})
	rec.On("wait-for", tmuxtest.Result{OnExec: func() { close(release) }})

	window := windowWithPane("w", enums.LayoutTiled, &decoders.Pane{
		Base:     &decoders.Base{Name: "slow"},
		Commands: []string{"sleep 600", "nvim"},
	})
	window.Panes = append(window.Panes, &decoders.Pane{Base: &decoders.Base{Name: "next"}, Commands: []string{"htop"}})

	assert.NoError(t, up.generateWindows([]*decoders.Window{window}, nil))

	assert.Contains(t, logs.String(), "glaze stopped waiting for the pane commands")
	assert.Contains(t, logs.String(), "name=slow")
	assert.Equal(t, 2, rec.CountOf("load-buffer"))
}

func TestActionUpStopsWhenCommandsCannotBeLoaded(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1;/tmp"})
	rec.On("load-buffer", tmuxtest.Result{Err: errors.New("no server")})

	window := windowWithPane("w", enums.LayoutTiled, &decoders.Pane{Base: &decoders.Base{Name: "p"}, Commands: []string{"htop"}})

	err := up.generateWindows([]*decoders.Window{window}, nil)
	assert.ErrorContains(t, err, "could not run the commands for pane `p` in window `w`")
}

func TestActionUpStopsWhenTheShellCannotBeRead(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("neww", tmuxtest.Result{Output: "@1;1;w;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1;/tmp"})
	rec.On("display-message", tmuxtest.Result{Err: errors.New("no server")})

	window := windowWithPane("w", enums.LayoutTiled, &decoders.Pane{Base: &decoders.Base{Name: "p"}, Commands: []string{"htop"}})

	assert.Error(t, up.generateWindows([]*decoders.Window{window}, nil))
	assert.False(t, rec.Called("load-buffer"))
}

func TestActionUpStopsWhenTheActivePaneCannotBeFound(t *testing.T) {
	up, rec := newTestUp(t)

	rec.On("lsw", tmuxtest.Result{Output: "@1;1;default;tiled;1"})
	rec.On("lsp", tmuxtest.Result{Output: "%1;1;default;1;/tmp"})
	rec.On("splitw", tmuxtest.Result{Output: "%2;1;p;1;/tmp"})
	rec.On("display-message", tmuxtest.Result{Err: errors.New("no server")})

	profile := &decoders.Session{Base: &decoders.Base{Name: "demo"}, Commands: []string{"htop"}}
	profile.Windows = []*decoders.Window{windowWithPane("w", enums.LayoutTiled, &decoders.Pane{Base: &decoders.Base{Name: "p"}})}

	err := up.provisionSession(profile)
	assert.ErrorContains(t, err, "could not find the active pane of session `demo`")
	assert.False(t, rec.Called("load-buffer"))
}

// commandsIn returns the last line of each loaded buffer, which runs the last command of each list.
func commandsIn(rec *tmuxtest.Recorder) []string {
	var last []string
	for _, input := range rec.InputsFor("load-buffer") {
		lines := strings.Split(strings.TrimSuffix(input, "\n"), "\n")
		last = append(last, lines[len(lines)-1])
	}
	return last
}

// lastCall returns the arguments of the last invocation of the given subcommand.
func lastCall(rec *tmuxtest.Recorder, subcommand string) []string {
	var last []string
	for _, call := range rec.Calls {
		if call[0] == subcommand {
			last = call
		}
	}
	return last
}
