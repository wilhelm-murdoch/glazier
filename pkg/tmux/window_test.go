package tmux

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

func TestWindowSplit(t *testing.T) {
	t.Run("successfully splits the window", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;buildhost;1;/srv"})
		rec.On("selectp", fakeResult{})

		client := testClient()
		window := testWindow(testSession(client))
		pane, err := window.Split("%1", "shell", "/srv")

		require.NoError(t, err)
		assert.Equal(t, PaneId(2), pane.Id)
		assert.Equal(t, "shell", pane.Name)
		assert.Equal(t, "/srv", pane.StartingDirectory)
		assert.True(t, pane.IsActive)
		assert.Subset(t, rec.ArgsFor("selectp"), []string{"-T", "shell", "-t", "%2"})
	})

	t.Run("uses the requested directory, not the path tmux reports", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;buildhost;1;/home/server"})
		rec.On("selectp", fakeResult{})

		pane, err := testWindow(testSession(testClient())).Split("%1", "shell", "/srv")
		require.NoError(t, err)
		assert.Equal(t, "/srv", pane.StartingDirectory)
	})

	t.Run("sanitises a backslash and control characters in the title", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;buildhost;1;/srv"})
		rec.On("selectp", fakeResult{})

		pane, err := testWindow(testSession(testClient())).Split("%1", "p\\z\tq", "/srv")
		require.NoError(t, err)
		assert.Equal(t, "p-z-q", pane.Name)
		assert.Subset(t, rec.ArgsFor("selectp"), []string{"-T", "p-z-q"})
	})

	t.Run("escapes format sequences in the directory and title", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;buildhost;1;/d#S"})
		rec.On("selectp", fakeResult{})

		window := testWindow(testSession(testClient()))
		_, err := window.Split("%1", "p#{pane_id}", "/d#S")
		assert.NoError(t, err)
		assert.Subset(t, rec.ArgsFor("splitw"), []string{"-c", "/d##S"})
		assert.Subset(t, rec.ArgsFor("selectp"), []string{"-T", "p##{pane_id}"})
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

	t.Run("errors on malformed pane response", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;1"})

		client := testClient()
		window := testWindow(testSession(client))
		_, err := window.Split("%1", "shell", "/srv")
		assert.ErrorIs(t, err, ErrUnexpectedPartCount)
		assert.False(t, rec.Called("selectp"))
	})

	t.Run("propagates select-pane errors", func(t *testing.T) {
		selectErr := errors.New("selectp failed")

		rec := setupRecorder(t)
		rec.On("splitw", fakeResult{Output: "%2;buildhost;1;/srv"})
		rec.On("selectp", fakeResult{Err: selectErr})

		client := testClient()
		window := testWindow(testSession(client))
		_, err := window.Split("%1", "shell", "/srv")
		assert.ErrorIs(t, err, selectErr)
	})
}

func TestWindowRename(t *testing.T) {
	t.Run("renames by id with the name sanitised and escaped", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("renamew", fakeResult{})

		window := testWindow(testSession(testClient()))
		window.Id = 7
		require.NoError(t, window.Rename("w#{x}\\y"))
		assert.Equal(t, []string{"renamew", "-t", "@7", "--", "w##{x}-y"}, rec.ArgsFor("renamew"))
		assert.Equal(t, "w#{x}-y", window.Name)
	})

	t.Run("keeps the old name when tmux fails", func(t *testing.T) {
		renameErr := errors.New("renamew failed")

		rec := setupRecorder(t)
		rec.On("renamew", fakeResult{Err: renameErr})

		window := testWindow(testSession(testClient()))
		assert.ErrorIs(t, window.Rename("new"), renameErr)
		assert.Equal(t, "win", window.Name)
	})
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
	for _, tc := range []struct{ name, layout string }{
		{"passes a preset", enums.LayoutTiled.String()},
		{"passes a raw layout string", "bb62,80x24,0,0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := setupRecorder(t)
			rec.On("selectl", fakeResult{})

			window := testWindow(testSession(testClient()))
			assert.NoError(t, window.SelectLayout(tc.layout))
			assert.Equal(t, []string{"selectl", "-t", "@0", tc.layout}, rec.ArgsFor("selectl"))
		})
	}
}

func TestWindowSetHook(t *testing.T) {
	t.Run("registers a window-scoped hook", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("set-hook", fakeResult{})

		client := testClient()
		window := testWindow(testSession(client))
		assert.NoError(t, window.SetHook("window-renamed", "echo renamed"))

		assert.Equal(t, []string{"set-hook", "-w", "-t", "@0", "--", "window-renamed", "echo renamed"}, rec.ArgsFor("set-hook"))
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

func TestWindowAutomaticName(t *testing.T) {
	for output, want := range map[string]bool{"1": true, "0": false} {
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: output})

		window := testWindow(testSession(testClient()))
		automatic, err := window.AutomaticName()
		assert.NoError(t, err)
		assert.Equal(t, want, automatic, output)
		assert.Subset(t, rec.ArgsFor("display-message"), []string{"-t", window.Target(), "#{automatic-rename}"})
	}

	t.Run("returns the error of tmux", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Err: errors.New("boom")})

		_, err := testWindow(testSession(testClient())).AutomaticName()
		assert.Error(t, err)
	})
}
