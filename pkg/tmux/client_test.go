package tmux

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

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
			expectedError:  `tmux: unexpected number of parts in line: expected 3, got 2: "$1;test-session-a"`,
			expectedValues: nil,
			sessionCount:   0,
		}, {
			name:           "fails with malformed expected return values from tmux client",
			cmdResponse:    "$a;;test-session-a+",
			expectedError:  `tmux: line has an invalid id: strconv.Atoi: parsing "a": invalid syntax`,
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

			assert.Equal(t, len(sessions), testCase.sessionCount)

			for _, value := range testCase.expectedValues {
				assert.True(t, slices.ContainsFunc(sessions, func(item *Session) bool {
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

		assert.ErrorIs(t, err, ErrUnreachable)
		assert.Nil(t, client)
	})

	t.Run("successfully finds tmux executable", func(t *testing.T) {
		_, err := NewClient(testSocketPath, testSocketPath, discardLogger)

		assert.Nil(t, err)
	})
}

// lookupCases are the tmux results that decide between a session or server that exists, one that does not and one that glaze cannot reach.
var lookupCases = []struct {
	name        string
	result      fakeResult
	exists      bool
	unreachable bool
}{
	{name: "exists", result: fakeResult{}, exists: true},
	{name: "session missing", result: tmuxFailure("can't find session: demos")},
	{name: "no socket file", result: tmuxFailure("error connecting to /tmp/tmux-1000/default (No such file or directory)")},
	{name: "stale socket", result: tmuxFailure("no server running on /tmp/tmux-1000/default")},
	{name: "permission denied", result: tmuxFailure("error connecting to /tmp/tmux-0/default (Permission denied)"), unreachable: true},
	{name: "tmux cannot run", result: fakeResult{Err: errors.New("fork/exec /usr/bin/tmux: exec format error")}, unreachable: true},
}

func TestClientIsRunning(t *testing.T) {
	for _, c := range lookupCases {
		t.Run(c.name, func(t *testing.T) {
			rec := setupRecorder(t)
			rec.On("list-sessions", c.result)

			running, err := testClient().IsRunning()
			assert.Equal(t, c.exists, running)
			if c.unreachable {
				assert.ErrorIs(t, err, ErrUnreachable)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClientNewSession(t *testing.T) {
	t.Run("successfully create a new session", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("new", fakeResult{Output: "$1;test;/foo/bar"})

		session, err := testClient().NewSession("test", "/foo/bar")

		assert.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, "$1", session.Id.String())
		assert.Equal(t, "test", session.Name)
		assert.Equal(t, "/foo/bar", session.StartingDirectory)
	})

	t.Run("reports a session that another client created first", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("new", tmuxFailure("duplicate session: test"))

		session, err := testClient().NewSession("test", "/foo/bar")
		assert.ErrorIs(t, err, ErrDuplicateSession)
		assert.Nil(t, session)
	})

	t.Run("fails when the primary new-session command errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("new", fakeResult{Err: errors.New("generic error message")})

		session, err := testClient().NewSession("test", "/foo/bar")

		assert.Error(t, err)
		assert.Nil(t, session)
		assert.Equal(t, "generic error message", err.Error())
	})

	t.Run("escapes format sequences in the name and directory", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("new", fakeResult{Output: "$1;s#{x};/d#S"})

		_, err := testClient().NewSession("s#{x}", "/d#S")
		assert.NoError(t, err)
		assert.Subset(t, rec.ArgsFor("new"), []string{"-s", "s##{x}", "-c", "/d##S"})
	})

	t.Run("sanitizes names tmux would rewrite so the session is findable", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("new", fakeResult{Output: "$1;my-app-1;/foo/bar"})

		session, err := testClient().NewSession("my.app:1", "/foo/bar")

		assert.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, "my-app-1", session.Name)
		assert.Contains(t, rec.ArgsFor("new"), "my-app-1")
	})
}

func TestSanitizeSessionName(t *testing.T) {
	for name, expected := range map[string]string{
		"plain":     "plain",
		"my.app":    "my-app",
		"my:app":    "my-app",
		"a.b:c.d":   "a-b-c-d",
		`back\sl`:   "back-sl",
		"p$x":       "p-x",
		"p${x}":     "p-{x}",
		"no_change": "no_change",
		"semi;co n": "semi;co n",
		"tab\tx":    "tab-x",
	} {
		assert.Equal(t, expected, SanitizeSessionName(name))
	}
}

func TestSanitizeName(t *testing.T) {
	for name, expected := range map[string]string{
		"plain":    "plain",
		`back\sl`:  "back-sl",
		"tab\tx":   "tab-x",
		"del\x7fx": "del-x",
		"a.b:c":    "a.b:c",
		"p$x":      "p$x",
		"ünï":      "ünï",
	} {
		assert.Equal(t, expected, SanitizeName(name))
	}
}

func TestClientKillSessionByName(t *testing.T) {
	t.Run("successfully kills a session", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("kill-session", fakeResult{})

		assert.NoError(t, testClient().KillSessionByName("demo"))

		args := rec.ArgsFor("kill-session")
		assert.Contains(t, args, "=demo")
	})

	t.Run("wraps the underlying error", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("kill-session", fakeResult{Err: errors.New("boom")})

		err := testClient().KillSessionByName("demo")
		assert.Error(t, err)
		assert.Equal(t, `session "demo" could not be killed: boom`, err.Error())

		args := rec.ArgsFor("kill-session")
		assert.Contains(t, args, "=demo")
	})
}

func TestClientFindSessionByName(t *testing.T) {
	t.Run("finds an existing session", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("ls", fakeResult{Output: "$1;demo;/tmp"})

		session, err := testClient().FindSessionByName("demo")
		assert.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, "demo", session.Name)
	})

	t.Run("errors when the session is missing", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("ls", fakeResult{Output: "$1;other;/tmp"})

		session, err := testClient().FindSessionByName("demo")
		assert.Error(t, err)
		assert.Nil(t, session)
		assert.Equal(t, `session "demo" not found`, err.Error())
	})
}

func TestClientHasSession(t *testing.T) {
	for _, c := range lookupCases {
		t.Run(c.name, func(t *testing.T) {
			rec := setupRecorder(t)
			rec.On("has-session", c.result)

			exists, err := testClient().HasSession("demos")
			assert.Equal(t, c.exists, exists)
			if c.unreachable {
				assert.ErrorIs(t, err, ErrUnreachable)
			} else {
				assert.NoError(t, err)
			}

			assert.Contains(t, rec.ArgsFor("has-session"), "=demos")
		})
	}
}

// insideServer sets the environment of a pane of the server on /tmp/tmux-1000/default.
func insideServer(t *testing.T, pane string) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	t.Setenv("TMUX_PANE", pane)
}

func TestClientInsideServer(t *testing.T) {
	t.Run("is false outside tmux, with no tmux call", func(t *testing.T) {
		t.Setenv("TMUX", "")
		rec := setupRecorder(t)

		inside, err := testClient().InsideServer()
		assert.NoError(t, err)
		assert.False(t, inside)
		assert.Empty(t, rec.Calls)
	})

	t.Run("is true when $TMUX names this server's socket", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})

		inside, err := testClient().InsideServer()
		assert.NoError(t, err)
		assert.True(t, inside)
		assert.Equal(t, []string{"display-message", "-p", "#{socket_path}"}, rec.ArgsFor("display-message"))
	})

	t.Run("is false when $TMUX names another server's socket", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/other"})

		inside, err := testClient().InsideServer()
		assert.NoError(t, err)
		assert.False(t, inside)
	})

	t.Run("is false when no server runs on this socket", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", tmuxFailure("no server running on /tmp/tmux-1000/other"))

		inside, err := testClient().InsideServer()
		assert.NoError(t, err)
		assert.False(t, inside)
	})

	t.Run("errors when tmux is unreachable", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", tmuxFailure("error connecting to /tmp/tmux-1000/default (Permission denied)"))

		_, err := testClient().InsideServer()
		assert.ErrorIs(t, err, ErrUnreachable)
	})
}

func TestClientCurrentSession(t *testing.T) {
	t.Run("returns the session of the pane that glaze runs in", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})
		rec.On("display-message", fakeResult{Output: "$1;demo;/tmp\n"})

		session, err := testClient().CurrentSession()
		assert.NoError(t, err)
		assert.Equal(t, "demo", session.Name)
		assert.Equal(t, SessionId(1), session.Id)
		assert.Equal(t, []string{"display-message", "-p", "-t", "%3", formatActiveSessions}, rec.Calls[1])
	})

	t.Run("is nil outside tmux", func(t *testing.T) {
		t.Setenv("TMUX", "")
		rec := setupRecorder(t)

		session, err := testClient().CurrentSession()
		assert.NoError(t, err)
		assert.Nil(t, session)
		assert.Empty(t, rec.Calls)
	})

	t.Run("is nil without $TMUX_PANE", func(t *testing.T) {
		insideServer(t, "")
		rec := setupRecorder(t)

		session, err := testClient().CurrentSession()
		assert.NoError(t, err)
		assert.Nil(t, session)
		assert.Empty(t, rec.Calls)
	})

	t.Run("is nil inside another tmux server", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/other"})

		session, err := testClient().CurrentSession()
		assert.NoError(t, err)
		assert.Nil(t, session)
		assert.Len(t, rec.Calls, 1)
	})

	t.Run("returns unparsable session result", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})
		rec.On("display-message", fakeResult{Output: "garbage"})

		_, err := testClient().CurrentSession()
		assert.ErrorIs(t, err, ErrUnexpectedPartCount)
	})

	t.Run("wraps the underlying error", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})
		rec.On("display-message", fakeResult{Err: errors.New("can't find pane")})

		_, err := testClient().CurrentSession()
		assert.ErrorContains(t, err, "could not determine current session")
	})
}

func TestClientWindows(t *testing.T) {
	t.Run("returns windows for a session", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("lsw", fakeResult{Output: "@1;1;win-a;tiled;1\n@2;2;win-b;even-vertical;0"})
		rec.On("show", fakeResult{Output: "base-index 1"})
		rec.On("show", fakeResult{Output: "base-index 1"})

		client := testClient()
		windows, err := client.Windows(testSession(client))

		assert.NoError(t, err)
		assert.Equal(t, 2, len(windows))
		assert.True(t, slices.ContainsFunc(windows, func(w *Window) bool {
			return w.Name == "win-a" && w.Layout == enums.LayoutTiled && w.IsActive
		}))
	})

	t.Run("propagates command errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("lsw", fakeResult{Err: errors.New("lsw failed")})

		client := testClient()
		windows, err := client.Windows(testSession(client))

		assert.Error(t, err)
		assert.Equal(t, 0, len(windows))
	})

	t.Run("errors on a malformed window line", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("lsw", fakeResult{Output: "@x;1;win;tiled;1"})

		client := testClient()
		_, err := client.Windows(testSession(client))
		assert.Error(t, err)
	})
}

func TestClientPanes(t *testing.T) {
	t.Run("returns panes for a window", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("lsp", fakeResult{Output: "%1;1;pane-a;1;/tmp\n%2;2;pane-b;0;/var"})
		rec.On("show", fakeResult{Output: "pane-base-index 1"})

		client := testClient()
		window := testWindow(testSession(client))
		panes, err := client.Panes(window)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(panes))
		assert.True(t, slices.ContainsFunc(panes, func(p *Pane) bool {
			return p.Name == "pane-a" && p.IsActive
		}))
	})

	t.Run("propagates list errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("lsp", fakeResult{Err: errors.New("lsp failed")})

		client := testClient()
		panes, err := client.Panes(testWindow(testSession(client)))

		assert.Error(t, err)
		assert.Equal(t, 0, len(panes))
	})

	t.Run("errors on a malformed pane line", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("lsp", fakeResult{Output: "%x;1;pane-a;1;/tmp"})
		rec.On("show", fakeResult{Output: "pane-base-index 1"})

		client := testClient()
		_, err := client.Panes(testWindow(testSession(client)))
		assert.Error(t, err)
	})

}

func TestClientNewWindowFromLine(t *testing.T) {
	t.Run("parses a valid window line", func(t *testing.T) {
		client := testClient()
		session := testSession(client)

		rec := setupRecorder(t)
		rec.On("show", fakeResult{Output: "base-index 1"})

		window, err := client.NewWindowFromLine("@3;1;editor;main-vertical;1", session)
		assert.NoError(t, err)
		assert.Equal(t, "@3", window.Id.String())
		assert.Equal(t, 1, window.Index)
		assert.Equal(t, "editor", window.Name)
		assert.Equal(t, enums.LayoutMainVertical, window.Layout)
		assert.True(t, window.IsActive)
	})

	client := testClient()
	session := testSession(client)
	t.Run("errors on a non-numeric index", func(t *testing.T) {
		_, err := client.NewWindowFromLine("@3;x;editor;tiled;1", session)
		assert.Error(t, err)
	})

	t.Run("errors on too few parts", func(t *testing.T) {
		_, err := client.NewWindowFromLine("@3;1;editor", session)
		assert.Error(t, err)
	})
}

func TestClientNewPaneFromLine(t *testing.T) {
	t.Run("parses a valid pane line", func(t *testing.T) {
		client := testClient()
		window := testWindow(testSession(client))

		rec := setupRecorder(t)
		rec.On("show", fakeResult{Output: "base-index 1"})

		pane, err := client.NewPaneFromLine("%4;1;shell;1;/srv", window)
		assert.NoError(t, err)
		assert.Equal(t, PaneId(4), pane.Id)
		assert.Equal(t, 1, pane.Index)
		assert.Equal(t, "shell", pane.Name)
		assert.Equal(t, "/srv", pane.StartingDirectory)
		assert.True(t, pane.IsActive)
	})

	t.Run("errors on a non-numeric index", func(t *testing.T) {
		client := testClient()
		window := testWindow(testSession(client))

		rec := setupRecorder(t)
		rec.On("show", fakeResult{Output: "base-index 1"})

		_, err := client.NewPaneFromLine("%4;x;shell;1;/srv", window)
		assert.Error(t, err)
	})

	t.Run("errors on too few parts", func(t *testing.T) {
		client := testClient()
		window := testWindow(testSession(client))

		rec := setupRecorder(t)
		rec.On("show", fakeResult{Output: "base-index 1"})

		_, err := client.NewPaneFromLine("%4;1;shell", window)
		assert.Error(t, err)
	})
}

func TestClientGetOption(t *testing.T) {
	t.Run("resolves a known scope", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("show", fakeResult{Output: "base-index 1"})

		out, err := testClient().GetOption("demo", "base-index", "global")
		assert.NoError(t, err)
		assert.Equal(t, "base-index 1", out)
		assert.Contains(t, rec.ArgsFor("show"), "-g")
	})

	t.Run("falls back to global scope for unknown scope", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("show", fakeResult{Output: "base-index 0"})

		out, err := testClient().GetOption("demo", "base-index", "bogus")
		assert.NoError(t, err)
		assert.Equal(t, "base-index 0", out)

		// Regression guard: an unknown scope must resolve to -g, never an empty
		// argument (tmux rejects an empty positional with "too many arguments").
		args := rec.ArgsFor("show")
		assert.Contains(t, args, "-g")
		assert.NotContains(t, args, "")
	})

	t.Run("propagates command errors", func(t *testing.T) {
		rec := setupRecorder(t)
		rec.On("show", fakeResult{Err: errors.New("show failed")})

		_, err := testClient().GetOption("demo", "base-index", "global")
		assert.Error(t, err)
	})
}

func TestClientAttach(t *testing.T) {
	t.Run("switches the client inside this server", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})
		rec.On("switchc", fakeResult{})

		client := testClient()
		session := testSession(client)
		session.Id = 7

		assert.NoError(t, client.Attach(session))
		assert.Subset(t, rec.ArgsFor("switchc"), []string{"-t", "$7"})
		assert.False(t, rec.Called("attach"))
	})

	t.Run("does not attach inside another tmux server", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/other"})

		client := testClient()
		assert.ErrorIs(t, client.Attach(testSession(client)), ErrOtherServer)
		assert.False(t, rec.Called("attach"))
		assert.False(t, rec.Called("switchc"))
	})

	t.Run("attaches outside tmux", func(t *testing.T) {
		t.Setenv("TMUX", "")
		rec := setupRecorder(t)
		rec.On("attach", fakeResult{})

		client := testClient()
		session := testSession(client)
		session.Id = 7
		assert.NoError(t, client.Attach(session))
		assert.Subset(t, rec.ArgsFor("attach"), []string{"-t", "$7"})
		assert.False(t, rec.Called("switchc"))
	})

	t.Run("errors when tmux is unreachable", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", tmuxFailure("error connecting to /tmp/tmux-1000/default (Permission denied)"))

		client := testClient()
		assert.ErrorIs(t, client.Attach(testSession(client)), ErrUnreachable)
	})

	t.Run("leaves the socket flags to NewCommand", func(t *testing.T) {
		insideServer(t, "%3")
		rec := setupRecorder(t)
		rec.On("display-message", fakeResult{Output: "/tmp/tmux-1000/default"})
		rec.On("switchc", fakeResult{})

		client := Client{socketName: "sock", socketPath: "/tmp/tmux.sock", logger: discardLogger}
		assert.NoError(t, client.Attach(testSession(client)))
		assert.Equal(t, []string{"switchc", "-t", "$0"}, rec.ArgsFor("switchc"))
	})

	t.Run("wraps attach errors", func(t *testing.T) {
		t.Setenv("TMUX", "")
		rec := setupRecorder(t)
		rec.On("attach", fakeResult{Err: errors.New("nope")})

		client := testClient()
		err := client.Attach(testSession(client))
		assert.ErrorContains(t, err, "demo")
	})
}

func TestClientAttachCommand(t *testing.T) {
	session := &Session{Name: "it's here"}

	assert.Equal(t, `tmux attach -t '=it'\''s here'`, Client{}.AttachCommand(session))
	assert.Equal(t, `tmux -L 'work' attach -t '=it'\''s here'`, Client{socketName: "work"}.AttachCommand(session))
	assert.Equal(t, `tmux -S '/tmp/my sock' attach -t '=it'\''s here'`, Client{socketPath: "/tmp/my sock"}.AttachCommand(session))
}
