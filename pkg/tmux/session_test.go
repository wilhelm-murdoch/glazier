package tmux

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

func TestSessionIdString(t *testing.T) {
	assert.Equal(t, "$7", SessionId(7).String())
}

func TestSessionTarget(t *testing.T) {
	client := testClient()
	assert.Equal(t, "demo", testSession(client).Target())
}

func TestSessionNewWindow(t *testing.T) {
	t.Run("successfully creates a window", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;1;editor;tiled;1"})
		rec.On("show", fakeResult{Output: "base-index 1"})

		client := testClient()
		window, err := testSession(client).NewWindow("editor", "")

		assert.NoError(t, err)
		assert.NotNil(t, window)
		assert.Equal(t, 1, window.Id)
		assert.Equal(t, 1, window.Index)
		assert.Equal(t, "editor", window.Name)
		assert.Equal(t, enums.LayoutTiled, window.Layout)
		assert.True(t, window.IsActive)
		assert.True(t, window.IsFirst)

		// No starting directory was given, so no -c flag should be sent.
		assert.NotContains(t, rec.ArgsFor("neww"), "-c")
	})

	t.Run("passes the starting directory with -c", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;1;editor;tiled;1"})
		rec.On("show", fakeResult{Output: "base-index 1"})

		client := testClient()
		_, err := testSession(client).NewWindow("editor", "/srv/app")
		assert.NoError(t, err)

		args := rec.ArgsFor("neww")
		assert.Contains(t, args, "-c")
		assert.Contains(t, args, "/srv/app")
	})

	t.Run("propagates command errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Err: errors.New("neww failed")})

		client := testClient()
		window, err := testSession(client).NewWindow("editor", "")

		assert.Error(t, err)
		assert.Nil(t, window)
	})

	t.Run("errors on non-numeric window id", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@x;1;editor;tiled;1"})

		client := testClient()
		_, err := testSession(client).NewWindow("editor", "")
		assert.Error(t, err)
	})

	t.Run("errors on malformed window response", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;1"})

		client := testClient()
		window, err := testSession(client).NewWindow("editor", "")
		assert.Error(t, err)
		assert.Nil(t, window)
		assert.Contains(t, err.Error(), "expected 5 fields")
	})

	t.Run("errors on non-numeric window index", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;x;editor;tiled;1"})

		client := testClient()
		_, err := testSession(client).NewWindow("editor", "")
		assert.Error(t, err)
	})

	t.Run("errors when base index lookup fails", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;1;editor;tiled;1"})
		rec.On("show", fakeResult{Err: errors.New("show failed")})

		client := testClient()
		_, err := testSession(client).NewWindow("editor", "")
		assert.Error(t, err)
	})

	t.Run("errors when base index is malformed", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;1;editor;tiled;1"})
		rec.On("show", fakeResult{Output: "base-index"})

		client := testClient()
		_, err := testSession(client).NewWindow("editor", "")
		assert.Error(t, err)
		assert.Equal(t, "could not determine window base index", err.Error())
	})
}

func TestSessionKill(t *testing.T) {
	t.Run("successfully kills the session", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("kill-session", fakeResult{})

		client := testClient()
		assert.NoError(t, testSession(client).Kill())
		assert.True(t, rec.Called("kill-session"))
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("kill-session", fakeResult{Err: errors.New("boom")})

		client := testClient()
		assert.Error(t, testSession(client).Kill())
	})
}

func TestSessionSetEnv(t *testing.T) {
	t.Run("sets an environment variable on the session", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("setenv", fakeResult{})

		client := testClient()
		assert.NoError(t, testSession(client).SetEnv("EDITOR", "vim"))

		args := rec.ArgsFor("setenv")
		assert.Contains(t, args, "demo")
		assert.Contains(t, args, "EDITOR")
		assert.Contains(t, args, "vim")
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("setenv", fakeResult{Err: errors.New("boom")})

		client := testClient()
		assert.Error(t, testSession(client).SetEnv("EDITOR", "vim"))
	})
}

func TestSessionSetHook(t *testing.T) {
	t.Run("registers a session hook", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-hook", fakeResult{})

		client := testClient()
		assert.NoError(t, testSession(client).SetHook("session-created", "echo hi"))

		args := rec.ArgsFor("set-hook")
		assert.Contains(t, args, "demo")
		assert.Contains(t, args, "session-created")
		assert.Contains(t, args, "echo hi")
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-hook", fakeResult{Err: errors.New("boom")})

		client := testClient()
		assert.Error(t, testSession(client).SetHook("session-created", "echo hi"))
	})
}

func TestSessionSetOption(t *testing.T) {
	t.Run("sets a session option", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-option", fakeResult{})

		client := testClient()
		assert.NoError(t, testSession(client).SetOption("base-index", "1"))

		args := rec.ArgsFor("set-option")
		assert.Contains(t, args, "demo")
		assert.Contains(t, args, "base-index")
		assert.Contains(t, args, "1")
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-option", fakeResult{Err: errors.New("boom")})

		client := testClient()
		assert.Error(t, testSession(client).SetOption("base-index", "1"))
	})
}

func TestSessionSendKeysAndWait(t *testing.T) {
	t.Run("sends keys with a wait-for signal and blocks", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send", fakeResult{})
		rec.On("wait-for", fakeResult{})

		client := testClient()
		assert.NoError(t, testSession(client).SendKeysAndWait("make build", "glaze-session-demo-0"))

		sendArgs := rec.ArgsFor("send")
		assert.Contains(t, sendArgs, "demo")
		assert.Contains(t, sendArgs, "make build ; tmux wait-for -S glaze-session-demo-0")
		assert.Equal(t, "Enter", sendArgs[len(sendArgs)-1])
		assert.Contains(t, rec.ArgsFor("wait-for"), "glaze-session-demo-0")
	})

	t.Run("propagates send errors without waiting", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send", fakeResult{Err: errors.New("send failed")})

		client := testClient()
		assert.Error(t, testSession(client).SendKeysAndWait("make build", "ch"))
		assert.False(t, rec.Called("wait-for"))
	})

	t.Run("propagates wait errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("send", fakeResult{})
		rec.On("wait-for", fakeResult{Err: errors.New("wait failed")})

		client := testClient()
		assert.Error(t, testSession(client).SendKeysAndWait("make build", "ch"))
	})
}
