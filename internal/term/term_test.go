package term

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColorEnabled(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create a pipe: %v", err)
	}

	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })

	t.Run("a pipe is not a terminal", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "xterm-256color")
		assert.False(t, ColorEnabled(w))
	})

	t.Run("NO_COLOR turns colour off", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		assert.False(t, ColorEnabled(os.Stderr))
	})

	t.Run("TERM=dumb turns colour off", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "dumb")
		assert.False(t, ColorEnabled(os.Stderr))
	})
}
