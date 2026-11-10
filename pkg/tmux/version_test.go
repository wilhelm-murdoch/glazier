package tmux

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		output string
		want   Version
		ok     bool
	}{
		{"tmux 3.2a", Version{3, 2, "a"}, true},
		{"tmux 3.7c\n", Version{3, 7, "c"}, true},
		{"tmux 3.4", Version{3, 4, ""}, true},
		{"tmux next-3.9", Version{3, 9, ""}, true},
		{"tmux 3.3a-rc2", Version{3, 3, "a"}, true},
		{"tmux 10.0", Version{10, 0, ""}, true},
		{"tmux openbsd-7.6", Version{}, false},
		{"3.2a", Version{}, false},
		{"", Version{}, false},
	}

	for _, c := range cases {
		t.Run(c.output, func(t *testing.T) {
			got, ok := ParseVersion(c.output)
			assert.Equal(t, c.ok, ok)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestVersionCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"tmux 3.2a", "tmux 3.2a", 0},
		{"tmux 3.2", "tmux 3.2a", -1},
		{"tmux 3.2b", "tmux 3.2a", 1},
		{"tmux 3.1c", "tmux 3.2a", -1},
		{"tmux 3.10", "tmux 3.9", 1},
		{"tmux 2.9a", "tmux 3.0", -1},
		{"tmux next-3.9", "tmux 3.7c", 1},
	}

	for _, c := range cases {
		t.Run(c.a+" vs "+c.b, func(t *testing.T) {
			a, _ := ParseVersion(c.a)
			b, _ := ParseVersion(c.b)
			assert.Equal(t, c.want, a.Compare(b))
			assert.Equal(t, -c.want, b.Compare(a))
		})
	}
}

func TestVersionString(t *testing.T) {
	assert.Equal(t, "3.2a", MinVersion.String())
	assert.Equal(t, "3.4", Version{Major: 3, Minor: 4}.String())
}

func TestClientVersion(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("-V", fakeResult{Output: "tmux 3.5a"})

	version, err := testClient().Version()
	assert.NoError(t, err)
	assert.Equal(t, Version{Major: 3, Minor: 5, Patch: "a"}, version)
	assert.Equal(t, []string{"-V"}, rec.ArgsFor("-V"))
}

func TestClientVersionUnknown(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("-V", fakeResult{Output: "tmux openbsd-7.6"})

	_, err := testClient().Version()
	assert.ErrorIs(t, err, ErrUnknownVersion)
	assert.Contains(t, err.Error(), "openbsd-7.6")
}

func TestClientVersionFails(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("-V", tmuxFailure("Permission denied"))

	_, err := testClient().Version()
	assert.ErrorIs(t, err, ErrUnreachable)
}

func TestCheckVersion(t *testing.T) {
	cases := []struct {
		output string
		err    error
	}{
		{"tmux 3.2a", nil},
		{"tmux 3.7c", nil},
		{"tmux next-3.9", nil},
		{"tmux 3.2", ErrOldTmux},
		{"tmux 3.1c", ErrOldTmux},
		{"tmux 2.9a", ErrOldTmux},
	}

	for _, c := range cases {
		t.Run(c.output, func(t *testing.T) {
			rec := setupRecorder(t)
			rec.On("-V", fakeResult{Output: c.output})

			err := testClient().CheckVersion()
			if c.err == nil {
				assert.NoError(t, err)
				return
			}

			assert.ErrorIs(t, err, c.err)
			assert.Contains(t, err.Error(), "glaze needs tmux 3.2a or later")
		})
	}
}

func TestCheckVersionWarnsForUnknownVersion(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("-V", fakeResult{Output: "tmux openbsd-7.6"})

	var logs bytes.Buffer
	client := Client{logger: slog.New(slog.NewTextHandler(&logs, nil))}

	assert.NoError(t, client.CheckVersion())
	assert.Contains(t, logs.String(), "could not read the tmux version")
	assert.Contains(t, logs.String(), "openbsd-7.6")
	assert.Contains(t, logs.String(), "minimum=3.2a")
}

func TestCheckVersionFails(t *testing.T) {
	rec := setupRecorder(t)
	rec.On("-V", tmuxFailure("Permission denied"))

	assert.ErrorIs(t, testClient().CheckVersion(), ErrUnreachable)
}
