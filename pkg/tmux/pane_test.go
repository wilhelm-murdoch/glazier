package tmux

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

func TestPaneIdString(t *testing.T) {
	assert.Equal(t, "%3", PaneId(3).String())
}

func TestPaneTarget(t *testing.T) {
	client := testClient()
	pane := testPane(testWindow(testSession(client)))
	assert.Equal(t, "demo:1.1", pane.Target())
}

func TestPaneSendKeys(t *testing.T) {
	t.Run("sends keys followed by Enter", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send", fakeResult{})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.NoError(t, pane.SendKeys("ls -la"))

		args := rec.ArgsFor("send")
		assert.Contains(t, args, "ls -la")
		assert.Equal(t, "Enter", args[len(args)-1])
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send", fakeResult{Err: errors.New("send failed")})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.Error(t, pane.SendKeys("ls"))
	})
}

func TestPaneSendKeysAndWait(t *testing.T) {
	t.Run("appends a wait-for signal and blocks on the channel", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send", fakeResult{})
		rec.On("wait-for", fakeResult{})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.NoError(t, pane.SendKeysAndWait("make build", "glaze-1-0"))

		sendArgs := rec.ArgsFor("send")
		assert.Contains(t, sendArgs, "make build ; tmux wait-for -S glaze-1-0")
		assert.Equal(t, "Enter", sendArgs[len(sendArgs)-1])

		assert.Contains(t, rec.ArgsFor("wait-for"), "glaze-1-0")
	})

	t.Run("propagates send errors without waiting", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send", fakeResult{Err: errors.New("send failed")})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.Error(t, pane.SendKeysAndWait("make build", "glaze-1-0"))
		assert.False(t, rec.Called("wait-for"))
	})

	t.Run("propagates wait errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send", fakeResult{})
		rec.On("wait-for", fakeResult{Err: errors.New("wait failed")})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.Error(t, pane.SendKeysAndWait("make build", "glaze-1-0"))
	})
}

func TestPaneSetEnv(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("setenv", fakeResult{})

	client := testClient()
	pane := testPane(testWindow(testSession(client)))
	assert.NoError(t, pane.SetEnv("FOO", "bar"))

	args := rec.ArgsFor("setenv")
	// env is scoped to the owning session, so the target is the session name.
	assert.Contains(t, args, "demo")
	assert.Contains(t, args, "FOO")
	assert.Contains(t, args, "bar")
}

func TestPaneSetHook(t *testing.T) {
	t.Run("registers a pane-scoped hook", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-hook", fakeResult{})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.NoError(t, pane.SetHook("pane-focus-in", "echo focus"))

		args := rec.ArgsFor("set-hook")
		assert.Contains(t, args, "-p")
		assert.Contains(t, args, "demo:1.1")
		assert.Contains(t, args, "pane-focus-in")
		assert.Contains(t, args, "echo focus")
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-hook", fakeResult{Err: errors.New("boom")})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.Error(t, pane.SetHook("pane-focus-in", "echo focus"))
	})
}

func TestPaneResize(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("resizep", fakeResult{})

	client := testClient()
	pane := testPane(testWindow(testSession(client)))
	assert.NoError(t, pane.Resize("80", "24"))

	args := rec.ArgsFor("resizep")
	assert.Contains(t, args, "80")
	assert.Contains(t, args, "24")
}

func TestPaneSetOption(t *testing.T) {
	t.Run("sets a pane-scoped option", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-option", fakeResult{})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.NoError(t, pane.SetOption("remain-on-exit", "on"))

		args := rec.ArgsFor("set-option")
		assert.Contains(t, args, "-p")
		assert.Contains(t, args, "demo:1.1")
		assert.Contains(t, args, "remain-on-exit")
		assert.Contains(t, args, "on")
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-option", fakeResult{Err: errors.New("boom")})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.Error(t, pane.SetOption("remain-on-exit", "on"))
	})
}

func TestPaneAdjust(t *testing.T) {
	t.Run("resizes in the given direction", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("resizep", fakeResult{})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.NoError(t, pane.Adjust(enums.AdjustmentLeft, "10"))

		args := rec.ArgsFor("resizep")
		assert.Contains(t, args, "demo:1.1")
		assert.Contains(t, args, "-L")
		assert.Contains(t, args, "10")
	})

	t.Run("errors on an unknown direction", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("resizep", fakeResult{})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		err := pane.Adjust(enums.AdjustmentUnknown, "10")
		assert.Error(t, err)
		assert.False(t, rec.Called("resizep"))
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("resizep", fakeResult{Err: errors.New("boom")})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.Error(t, pane.Adjust(enums.AdjustmentUp, "5"))
	})
}

func TestPaneSelect(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("selectp", fakeResult{})

	client := testClient()
	pane := testPane(testWindow(testSession(client)))
	assert.NoError(t, pane.Select())
	assert.True(t, rec.Called("selectp"))
}

func TestPaneKill(t *testing.T) {
	t.Run("successfully kills the pane", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("killp", fakeResult{})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.NoError(t, pane.Kill())
		assert.True(t, rec.Called("killp"))
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("killp", fakeResult{Err: errors.New("killp failed")})

		client := testClient()
		pane := testPane(testWindow(testSession(client)))
		assert.Error(t, pane.Kill())
	})
}
