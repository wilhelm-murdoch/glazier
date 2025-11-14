package tmux

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

// MockCommander is a mocked Command struct that satisfies the Commander interface.
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
	funcTeardown   func(t *testing.T)
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

	newCommand = func(client Client, args ...string) Commander {
		deps.capturedArgs = args
		return deps.mockExec
	}
	originalNewCommand := newCommand
	t.Cleanup(func() {
		newCommand = originalNewCommand
	})

	return deps
}
