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
	assert.Equal(t, "$0", testSession(client).Target())
}

func TestSessionNewWindow(t *testing.T) {
	t.Run("sanitises a backslash and control characters in the name", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;1;w-z-q;tiled;1"})

		_, err := testSession(testClient()).NewWindow("w\\z\tq", "")
		assert.NoError(t, err)
		assert.Subset(t, rec.ArgsFor("neww"), []string{"-n", "w-z-q"})
	})

	t.Run("escapes format sequences in the name and directory", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;1;w#{session_name};tiled;1"})

		_, err := testSession(testClient()).NewWindow("w#{session_name}", "/d#S")
		assert.NoError(t, err)
		assert.Subset(t, rec.ArgsFor("neww"), []string{"-n", "w##{session_name}", "-c", "/d##S"})
	})

	t.Run("successfully creates a window", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;1;editor;tiled;1"})

		client := testClient()
		window, err := testSession(client).NewWindow("editor", "")

		assert.NoError(t, err)
		assert.NotNil(t, window)
		assert.Equal(t, "@1", window.Id.String())
		assert.Equal(t, 1, window.Index)
		assert.Equal(t, "editor", window.Name)
		assert.Equal(t, enums.LayoutTiled, window.Layout)
		assert.True(t, window.IsActive)

		// No starting directory was given, so no -c flag should be sent.
		assert.NotContains(t, rec.ArgsFor("neww"), "-c")
	})

	t.Run("passes the starting directory with -c", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;1;editor;tiled;1"})

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
		assert.ErrorIs(t, err, ErrUnexpectedPartCount)
	})

	t.Run("errors on non-numeric window index", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("neww", fakeResult{Output: "@1;x;editor;tiled;1"})

		client := testClient()
		_, err := testSession(client).NewWindow("editor", "")
		assert.Error(t, err)
	})

}

func TestSessionSetEnv(t *testing.T) {
	t.Run("sets an environment variable on the session", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("setenv", fakeResult{})

		client := testClient()
		assert.NoError(t, testSession(client).SetEnv("EDITOR", "vim"))

		args := rec.ArgsFor("setenv")
		assert.Contains(t, args, "$0")
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
		assert.Contains(t, args, "$0")
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
		assert.Contains(t, args, "$0")
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

func TestSessionActivePane(t *testing.T) {
	t.Run("returns the id of the active pane", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "%7"})

		pane, err := testSession(testClient()).ActivePane()
		assert.NoError(t, err)
		assert.Equal(t, "%7", pane)
		assert.Equal(t, []string{"display-message", "-p", "-t", "$0", "#{pane_id}"}, rec.ArgsFor("display-message"))
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Err: errors.New("no server")})

		_, err := testSession(testClient()).ActivePane()
		assert.Error(t, err)
	})
}

func TestSessionHost(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("display-message", fakeResult{Output: "buildhost"})

	host, err := testSession(testClient()).Host()
	assert.NoError(t, err)
	assert.Equal(t, "buildhost", host)
	assert.Contains(t, rec.ArgsFor("display-message"), "#{host}")
}
