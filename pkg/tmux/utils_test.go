package tmux

import (
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
)

const (
	testSocketPath = ""
	testSocketName = ""
)

// discardLogger is a logger that swallows all output so tests stay quiet.
var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// MockCommander is a mocked Command struct that satisfies the Commander interface.
// It is kept for simple, single-command tests where call routing is not needed.
type MockCommander struct {
	mock.Mock
}

func (m *MockCommander) Exec() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockCommander) ExecWithOutput() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockCommander) ExecWithStatus() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockCommander) String() string {
	args := m.Called()
	return args.String(0)
}

type TestCase struct {
	name           string
	funcSetup      func(t *testing.T)
	cmdResponse    string
	cmdError       error
	expectedError  string
	expectedValues [][]string
	sessionCount   int
}

type TestSuite struct {
	Cases []TestCase
}

func NewTestSuite(testCases []TestCase) *TestSuite {
	return &TestSuite{
		Cases: testCases,
	}
}

func (ts *TestSuite) Append(testCase TestCase) *TestSuite {
	ts.Cases = append(ts.Cases, testCase)
	return ts
}

func (ts *TestSuite) Extend(testCases []TestCase) *TestSuite {
	ts.Cases = append(ts.Cases, testCases...)
	return ts
}

type TestDepsBase struct {
	capturedArgs []string
	mockExec     *MockCommander
}

func setupTestDeps(t *testing.T) *TestDepsBase {
	deps := &TestDepsBase{
		mockExec: new(MockCommander),
	}

	deps.mockExec.On("String").Return("mocked tmux command")

	originalNewCommand := newCommand
	newCommand = func(client Client, args ...string) Commander {
		deps.capturedArgs = args
		return deps.mockExec
	}
	t.Cleanup(func() {
		newCommand = originalNewCommand
	})

	return deps
}

// fakeResult is the canned outcome a single faked tmux invocation returns.
type fakeResult struct {
	Output string
	Err    error
	Status int
}

// fakeCommand is a programmable Commander representing one tmux invocation.
// Unlike MockCommander it is value-driven, so a CommandRecorder can hand out a
// distinct result for every command a method issues.
type fakeCommand struct {
	args   []string
	result fakeResult
}

func (f *fakeCommand) String() string                  { return strings.Join(f.args, " ") }
func (f *fakeCommand) Exec() error                     { return f.result.Err }
func (f *fakeCommand) ExecWithOutput() (string, error) { return f.result.Output, f.result.Err }
func (f *fakeCommand) ExecWithStatus() int             { return f.result.Status }

// CommandRecorder records every tmux invocation made through newCommand and
// routes canned results based on the tmux subcommand (e.g. "ls", "neww",
// "splitw"). This lets a single test exercise methods that chain several
// different tmux commands, which the shared MockCommander cannot do.
type CommandRecorder struct {
	mu          sync.Mutex
	Calls       [][]string
	queues      map[string][]fakeResult
	fallback    fakeResult
	hasFallback bool
}

func NewCommandRecorder() *CommandRecorder {
	return &CommandRecorder{queues: make(map[string][]fakeResult)}
}

// On enqueues a canned result for the given tmux subcommand. Repeated calls for
// the same subcommand are returned in FIFO order.
func (r *CommandRecorder) On(subcommand string, result fakeResult) *CommandRecorder {
	r.queues[subcommand] = append(r.queues[subcommand], result)
	return r
}

// Default sets the result returned when no queued result matches a subcommand.
func (r *CommandRecorder) Default(result fakeResult) *CommandRecorder {
	r.fallback = result
	r.hasFallback = true
	return r
}

func (r *CommandRecorder) resultFor(args []string) fakeResult {
	sub := subcommandOf(args)
	if q := r.queues[sub]; len(q) > 0 {
		res := q[0]
		r.queues[sub] = q[1:]
		return res
	}
	return r.fallback
}

// Called reports whether the given tmux subcommand was ever invoked.
func (r *CommandRecorder) Called(subcommand string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.Calls {
		if subcommandOf(c) == subcommand {
			return true
		}
	}
	return false
}

// ArgsFor returns the arguments of the first invocation of the given subcommand.
func (r *CommandRecorder) ArgsFor(subcommand string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.Calls {
		if subcommandOf(c) == subcommand {
			return c
		}
	}
	return nil
}

// subcommandOf returns the tmux subcommand, skipping any leading socket flags
// (-L name / -S path) so routing works regardless of how args were assembled.
func subcommandOf(args []string) string {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-L", "-S":
			i++
			continue
		}
		return args[i]
	}
	return ""
}

// setupRecorder installs a CommandRecorder-backed newCommand factory and
// restores the original factory when the test finishes.
func setupRecorder(t *testing.T) *CommandRecorder {
	rec := NewCommandRecorder()

	originalNewCommand := newCommand
	newCommand = func(client Client, args ...string) Commander {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		rec.Calls = append(rec.Calls, args)
		return &fakeCommand{args: args, result: rec.resultFor(args)}
	}
	t.Cleanup(func() {
		newCommand = originalNewCommand
	})

	return rec
}

// testClient builds a Client wired to the discard logger without requiring a
// real tmux binary on PATH (fields are package-private, tests are in-package).
func testClient() Client {
	return Client{logger: discardLogger}
}

// testSession builds a Session attached to the given client.
func testSession(c Client) *Session {
	return &Session{Client: c, Name: "demo", StartingDirectory: "/tmp", logger: discardLogger}
}

// testWindow builds a Window attached to the given session.
func testWindow(s *Session) *Window {
	return &Window{Session: s, Index: 1, Name: "win"}
}

// testPane builds a Pane attached to the given window.
func testPane(w *Window) *Pane {
	return &Pane{Window: w, Index: 1, Name: "pane"}
}
