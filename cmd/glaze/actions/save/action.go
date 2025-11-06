package save

import (
	"log/slog"

	"github.com/urfave/cli/v2"

	"github.com/wilhelm-murdoch/glazier/cmd/glaze/actions"
)

type Action struct {
	actions.BaseAction
}

// NewAction is responsible for creating a new Action instance for the save command.
func NewAction(ctx *cli.Context, logger *slog.Logger) (*Action, error) {
	base, err := actions.NewBaseAction(ctx, logger)
	if err != nil {
		return nil, err
	}

	return &Action{
		BaseAction: *base,
	}, nil
}
