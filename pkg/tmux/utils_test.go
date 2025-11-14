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

	newCommand = func(client Client, args ...string) (Commander, error) {
		deps.capturedArgs = args
		return deps.mockExec, nil
	}

	t.Cleanup(func() {
		newCommand = originalNewCommand
	})

	return deps
}
