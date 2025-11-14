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

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type TestDepsClient struct {
	*TestDepsBase
	Client *Client
}

func setupClientTestDeps(t *testing.T) (*TestDepsClient, error) {
	base := setupTestDeps(t)

	client, err := NewClient(testSocketPath, testSocketName, discardLogger)
	if err != nil {
		return nil, err
	}

	return &TestDepsClient{
		TestDepsBase: base,
		Client:       client,
	}, nil
}

func TestClientSessions(t *testing.T) {
	testCases := []TestCase{
		{
			name:          "successfully returns multiple sessions",
			cmdResponse:   "$1;test-session-a;/tmp/foo\n$2;test-session-b;/tmp/bar\n$3;test-session-c;/tmp/baz",
			expectedError: "",
			expectedValues: [][]string{
				{"test-session-a", "/tmp/foo"},
				{"test-session-b", "/tmp/bar"},
				{"test-session-c", "/tmp/baz"},
			},
			sessionCount: 3,
		}, {
			name:          "successfully returns single session",
			cmdResponse:   "$1;test-session-a;/tmp/foo",
			expectedError: "",
			expectedValues: [][]string{
				{"test-session-a", "/tmp/foo"},
			},
			sessionCount: 1,
		}, {
			name:           "fails with missing expected return values from tmux client",
			cmdResponse:    "$1;test-session-a",
			expectedError:  "expected 3 parts for session line, but got 2 instead: $1;test-session-a",
			expectedValues: nil,
			sessionCount:   0,
		}, {
			name:           "fails with malformed expected return values from tmux client",
			cmdResponse:    "$a;;test-session-a+",
			expectedError:  "strconv.Atoi: parsing \"a\": invalid syntax",
			expectedValues: nil,
			sessionCount:   0,
		}, {
			name:           "fails to return sessions from tmux client",
			cmdResponse:    "",
			cmdError:       errors.New("command failed"),
			expectedError:  "command failed",
			expectedValues: nil,
			sessionCount:   0,
		},
	}

	testSuite := NewTestSuite(testCases)

	for _, testCase := range testSuite.Cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.funcSetup != nil {
				testCase.funcSetup(t)
			}

			deps, err := setupClientTestDeps(t)
			assert.NoError(t, err)
			assert.NotNil(t, deps)

			deps.mockExec.On("ExecWithOutput").Return(testCase.cmdResponse, testCase.cmdError)

			sessions, err := deps.Client.Sessions()

			if testCase.expectedError != "" {
				assert.Error(t, err)
				assert.Equal(t, testCase.expectedError, err.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, sessions.Length(), testCase.sessionCount)

			for _, value := range testCase.expectedValues {
				assert.NotNil(t, sessions.Find(func(i int, item *Session) bool {
					return item.Name == value[0] && item.StartingDirectory == value[1]
				}))
			}
		})
	}
}

func TestClientNew(t *testing.T) {
	t.Run("fails to find tmux executable", func(t *testing.T) {
		originalDefaultTmuxExecutablePath := defaultTmuxExecutablePath
		defaultTmuxExecutablePath = "/this/path/does/not/work/tmux"
		t.Cleanup(func() {
			defaultTmuxExecutablePath = originalDefaultTmuxExecutablePath
		})

		client, err := NewClient(testSocketPath, testSocketPath, discardLogger)

		assert.Error(t, err)
		assert.Equal(t, err.Error(), "tmux is not installed")
		assert.Nil(t, client)
	})

	t.Run("successfully finds tmux executable", func(t *testing.T) {
		_, err := NewClient(testSocketPath, testSocketPath, discardLogger)

		assert.Nil(t, err)
	})
}

func TestClientIsRunning(t *testing.T) {
	testCases := []struct {
		name           string
		exitStatus     int
		expectedResult bool
	}{
		{
			name:           "simulate tmux server is running",
			exitStatus:     0,
			expectedResult: true,
		},
		{
			name:           "simulate tmux server has stopped",
			exitStatus:     1,
			expectedResult: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deps, err := setupClientTestDeps(t)
			assert.NoError(t, err)
			assert.NotNil(t, deps)

			deps.mockExec.On("ExecWithStatus").Return(testCase.exitStatus)

			assert.Equal(t, deps.Client.IsRunning(), testCase.expectedResult)
		})
	}
}

func TestClientAttach(t *testing.T) {
	testCases := []struct {
		name string
	}{
		{
			name: "simulate tmux server is running",
		},
		{
			name: "simulate tmux server has stopped",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deps, err := setupClientTestDeps(t)
			assert.NoError(t, err)
			assert.NotNil(t, deps)
		})
	}
}
