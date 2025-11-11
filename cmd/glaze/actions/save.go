package actions

import (
	"github.com/urfave/cli/v3"
)

// ActionSave is a struct that represents a Glazier "action".
type ActionSave struct {
	ActionBase
}

// NewSave is responsible for creating a new ActionFormat struct value pre-populated
// with fields that are common across all other action structs.
func NewSave(cmd *cli.Command, logLevel string) (*ActionSave, error) {
	base, err := NewActionBase(cmd, logLevel)
	if err != nil {
		return nil, err
	}

	return &ActionSave{
		ActionBase: *base,
	}, nil
}
