package tmux

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testSocketPath = ""
	testSocketName = ""
)

var testDiscardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type ClientTestDeps struct {
	capturedArgs []string
	mockExec     *MockCommander
	client       *Client
}

func setupClientTest(t *testing.T) *ClientTestDeps {
	client := NewClient(testSocketPath, testSocketName, testDiscardLogger)

	deps := &ClientTestDeps{
		mockExec: new(MockCommander),
		client:   client,
	}

	deps.mockExec.On("String").Return("mocked tmux command")

	// originalIsInsideTmux := isInsideTmux
	originalNewCommand := newCommand

	t.Cleanup(func() {
		newCommand = originalNewCommand
		// isInsideTmux = originalIsInsideTmux
	})

	newCommand = func(client Client, args ...string) (Commander, error) {
		deps.capturedArgs = args
		return deps.mockExec, nil
	}

	return deps
}

func TestClientSessions_Success(t *testing.T) {
	deps := setupClientTest(t)

	deps.mockExec.On("ExecWithOutput").Return("$1;session-a;~/\n$2;session-b;~/", nil)

	sessions, err := deps.client.Sessions()

	assert.NoError(t, err)
	assert.Equal(t, sessions.Length(), 2)
	assert.Equal(t, "session-a", sessions.Items()[0].Name)
	assert.Equal(t, "~/", sessions.Items()[1].StartingDirectory)
}

func TestClientSessions_Failed(t *testing.T) {
	deps := setupClientTest(t)

	deps.mockExec.On("ExecWithOutput").Return("", errors.New("beep"))

	_, err := deps.client.Sessions()

	assert.Error(t, err)
}
