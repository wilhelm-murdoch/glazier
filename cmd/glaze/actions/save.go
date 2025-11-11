package actions

import (
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
)

// ActionSave is a struct that represents a Glazier "action".
type ActionSave struct {
	ActionBase
}

// NewSave is responsible for creating a new ActionFormat struct value pre-populated
// with fields that are common across all other action structs.
func NewSave(cmd *cli.Command, logger *logger.Logger) (*ActionSave, error) {
	base, err := NewActionBase(cmd, logger)
	if err != nil {
		return nil, err
	}

	return &ActionSave{
		ActionBase: *base,
	}, nil
}
