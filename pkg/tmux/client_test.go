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

type TestDepsClient struct {
	*TestDepsBase
	Client *Client
}

func setupClientTestDeps(t *testing.T) *TestDepsClient {
	base := setupTestDeps(t)
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewClient(testSocketPath, testSocketName, discardLogger)

	return &TestDepsClient{
		TestDepsBase: base,
		Client:       client,
	}
}

func TestClientSessions(t *testing.T) {
	testCases := []struct {
		name           string
		cmdResponse    string
		cmdError       error
		expectedError  string
		expectedValues [][]string
		sessionCount   int
	}{
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

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deps := setupClientTestDeps(t)
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

	t.Run("fails to find tmux executable", func(t *testing.T) {
		originalDefaultTmuxExecutablePath := defaultTmuxExecutablePath
		t.Cleanup(func() {
			defaultTmuxExecutablePath = originalDefaultTmuxExecutablePath
		})
		deps := setupClientTestDeps(t)
		deps.mockExec.On("ExecWithOutput").Return("", nil)
		defaultTmuxExecutablePath = "/an/invalid/path/to/tmux"

		_, err := deps.Client.Sessions()

		assert.Error(t, err)
	})
}
