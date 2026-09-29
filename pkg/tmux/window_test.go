package tmux

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

func TestWindowIdString(t *testing.T) {
	assert.Equal(t, "@5", WindowId(5).String())
}

func TestWindowTarget(t *testing.T) {
	client := testClient()
	window := testWindow(testSession(client))
	assert.Equal(t, "@0", window.Target())
}

func TestWindowSplit(t *testing.T) {
	t.Run("successfully splits the window", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;1;buildhost;1;/srv"})
		rec.On("show", fakeResult{Output: "pane-base-index 1"})
		rec.On("selectp", fakeResult{})

		client := testClient()
		window := testWindow(testSession(client))
		pane, err := window.Split("%1", "shell", "/srv")

		require.NoError(t, err)
		assert.Equal(t, PaneId(2), pane.Id)
		assert.Equal(t, 1, pane.Index)
		assert.Equal(t, "shell", pane.Name)
		assert.Equal(t, "/srv", pane.StartingDirectory)
		assert.True(t, pane.IsActive)
		assert.True(t, pane.IsFirst)
		assert.Subset(t, rec.ArgsFor("selectp"), []string{"-T", "shell", "-t", "%2"})
	})

	t.Run("propagates split command errors", func(t *testing.T) {
		splitErr := errors.New("splitw failed")

		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Err: splitErr})

		client := testClient()
		window := testWindow(testSession(client))
		_, err := window.Split("%1", "shell", "/srv")
		assert.ErrorIs(t, err, splitErr)
	})

	t.Run("propagates base index lookup errors", func(t *testing.T) {
		showErr := errors.New("show failed")

		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;1;buildhost;1;/srv"})
		rec.On("show", fakeResult{Err: showErr})

		client := testClient()
		window := testWindow(testSession(client))
		_, err := window.Split("%1", "shell", "/srv")
		assert.ErrorIs(t, err, showErr)
	})

	t.Run("errors when base index is malformed", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;1;buildhost;1;/srv"})
		rec.On("show", fakeResult{Output: "pane-base-index"})

		client := testClient()
		window := testWindow(testSession(client))
		_, err := window.Split("%1", "shell", "/srv")
		assert.EqualError(t, err, "could not determine pane base index")
	})

	t.Run("errors on malformed pane response", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;1"})
		rec.On("show", fakeResult{Output: "pane-base-index 1"})

		client := testClient()
		window := testWindow(testSession(client))
		_, err := window.Split("%1", "shell", "/srv")
		assert.ErrorIs(t, err, ErrUnexpectedPartCount)
	})

	t.Run("errors on non-numeric pane id", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%x;1;buildhost;1;/srv"})
		rec.On("show", fakeResult{Output: "pane-base-index 1"})

		client := testClient()
		window := testWindow(testSession(client))
		_, err := window.Split("%1", "shell", "/srv")
		assert.ErrorIs(t, err, ErrInvalidDerivedId)
	})

	t.Run("errors on non-numeric pane index", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;x;buildhost;1;/srv"})
		rec.On("show", fakeResult{Output: "pane-base-index 1"})

		client := testClient()
		window := testWindow(testSession(client))
		_, err := window.Split("%1", "shell", "/srv")
		assert.ErrorIs(t, err, strconv.ErrSyntax)
	})

	t.Run("propagates select-pane errors", func(t *testing.T) {
		selectErr := errors.New("selectp failed")

		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;1;buildhost;1;/srv"})
		rec.On("show", fakeResult{Output: "pane-base-index 1"})
		rec.On("selectp", fakeResult{Err: selectErr})

		client := testClient()
		window := testWindow(testSession(client))
		_, err := window.Split("%1", "shell", "/srv")
		assert.ErrorIs(t, err, selectErr)
	})
}

func TestWindowKill(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("killw", fakeResult{})

	client := testClient()
	window := testWindow(testSession(client))
	assert.NoError(t, window.Kill())
	assert.True(t, rec.Called("killw"))
}

func TestWindowSelect(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("selectw", fakeResult{})

	client := testClient()
	window := testWindow(testSession(client))
	assert.NoError(t, window.Select())
	assert.True(t, rec.Called("selectw"))
}

func TestWindowSelectLayout(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("selectl", fakeResult{})

	client := testClient()
	window := testWindow(testSession(client))
	assert.NoError(t, window.SelectLayout(enums.LayoutTiled.String()))
	assert.Contains(t, rec.ArgsFor("selectl"), "tiled")
}

func TestWindowSelectLayoutRawString(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("selectl", fakeResult{})

	client := testClient()
	window := testWindow(testSession(client))
	assert.NoError(t, window.SelectLayout("bb62,80x24,0,0"))
	assert.Contains(t, rec.ArgsFor("selectl"), "bb62,80x24,0,0")
}

func TestWindowSetHook(t *testing.T) {
	t.Run("registers a window-scoped hook", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-hook", fakeResult{})

		client := testClient()
		window := testWindow(testSession(client))
		assert.NoError(t, window.SetHook("window-renamed", "echo renamed"))

		args := rec.ArgsFor("set-hook")
		assert.Contains(t, args, "-w")
		assert.Contains(t, args, "@0")
		assert.Contains(t, args, "window-renamed")
		assert.Contains(t, args, "echo renamed")
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-hook", fakeResult{Err: errors.New("boom")})

		client := testClient()
		window := testWindow(testSession(client))
		assert.Error(t, window.SetHook("window-renamed", "echo renamed"))
	})
}

func TestWindowSetOption(t *testing.T) {
	t.Run("sets a window-scoped option", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-option", fakeResult{})

		client := testClient()
		window := testWindow(testSession(client))
		assert.NoError(t, window.SetOption("automatic-rename", "off"))

		args := rec.ArgsFor("set-option")
		assert.Contains(t, args, "-w")
		assert.Contains(t, args, "@0")
		assert.Contains(t, args, "automatic-rename")
		assert.Contains(t, args, "off")
	})

	t.Run("propagates errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-option", fakeResult{Err: errors.New("boom")})

		client := testClient()
		window := testWindow(testSession(client))
		assert.Error(t, window.SetOption("automatic-rename", "off"))
	})
}
