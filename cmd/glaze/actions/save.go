package actions

import (
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
)

type ActionSave struct {
	ActionBase
}

// NewAction is responsible for creating a new Action instance for the save command.
func NewSave(cmd *cli.Command, logger *logger.Logger) (*ActionSave, error) {
	base, err := NewActionBase(cmd, logger)
	if err != nil {
		return nil, err
	}

	return &ActionSave{
		ActionBase: *base,
	}, nil
}
