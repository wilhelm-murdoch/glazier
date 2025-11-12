package tmux

import "github.com/stretchr/testify/mock"

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
