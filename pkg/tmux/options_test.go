package tmux

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientOptionTables(t *testing.T) {
	t.Run("reads the session and window tables", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("show-options", fakeResult{Output: "base-index 0\nhistory-limit 2000\nstatus-format[0] \"#[align=left]\"\nstatus-format[1] x"})
		rec.On("show-options", fakeResult{Output: "remain-on-exit off\nautomatic-rename on\npane-border-style default"})

		tables, err := testClient().OptionTables()
		require.NoError(t, err)

		assert.True(t, tables.IsSessionOnly("history-limit"))
		assert.True(t, tables.IsSessionOnly("status-format"))
		assert.True(t, tables.IsWindowOnly("remain-on-exit"))
		assert.True(t, tables.IsWindowOnly("pane-border-style"))
		assert.False(t, tables.IsWindowOnly("history-limit"))
		assert.False(t, tables.IsSessionOnly("remain-on-exit"))
		assert.False(t, tables.IsSessionOnly("@user"))
		assert.False(t, tables.IsWindowOnly("@user"))

		assert.Equal(t, [][]string{{"show-options", "-g"}, {"show-options", "-gw"}}, rec.Calls)
	})

	t.Run("propagates a show-options error", func(t *testing.T) {
		showErr := errors.New("show-options failed")

		rec := setupRecorder(t)
		rec.On("show-options", fakeResult{Err: showErr})

		_, err := testClient().OptionTables()
		assert.ErrorIs(t, err, showErr)
	})
}
