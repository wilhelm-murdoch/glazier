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
	assert.Equal(t, "%0", pane.Target())
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
		assert.Contains(t, args, "%0")
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
	cases := []struct {
		name, x, y string
		want       []string
	}{
		{"both axes", "80", "24", []string{"resizep", "-t", "%0", "-x", "80", "-y", "24"}},
		{"width only", "25%", "", []string{"resizep", "-t", "%0", "-x", "25%"}},
		{"height only", "", "5", []string{"resizep", "-t", "%0", "-y", "5"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := setupRecorder(t)

			pane := testPane(testWindow(testSession(testClient())))
			assert.NoError(t, pane.Resize(c.x, c.y))
			assert.Equal(t, c.want, rec.ArgsFor("resizep"))
		})
	}
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
		assert.Contains(t, args, "%0")
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
		assert.Contains(t, args, "%0")
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
