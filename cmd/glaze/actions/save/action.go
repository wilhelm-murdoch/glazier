package save

import (
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
)

type Action struct {
	actions.BaseAction
}

// NewAction is responsible for creating a new Action instance for the save command.
func NewAction(cmd *cli.Command, logger *logger.Logger) (*Action, error) {
	base, err := actions.NewBaseAction(cmd, logger)
	if err != nil {
		return nil, err
	}

	return &Action{
		BaseAction: *base,
	}, nil
}
