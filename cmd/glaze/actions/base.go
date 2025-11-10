package actions

import (
	"context"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	ge "github.com/wilhelm-murdoch/glazier/internal/errors" // ge = "Glaze Errors"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
)

type BaseAction struct {
	Context            context.Context
	Command            *cli.Command
	DiagnosticsManager *diagnostics.DiagnosticsManager
	Parser             *parser.Parser
	ProfilePath        string
	Logger             *logger.Logger
}

// NewBaseAction is responsible for creating a new BaseAction instance, resolving the profile path, and initializing the diagnostics manager and parser.
func NewBaseAction(
	cmd *cli.Command,
	logger *logger.Logger,
) (*BaseAction, error) {
	profilePath, err := files.ResolveProfilePath(cmd.String("profile-path"))
	if err != nil {
		return nil, err
	}

	diagsManager := diagnostics.New(profilePath)
	if diagsManager.HasErrors() {
		diagsManager.Write()
		return nil, ge.ErrorInvalidDefinition
	}

	parser, parserDiags := parser.New(profilePath)
	if parserDiags.HasErrors() {
		diagsManager.Write()
		return nil, ge.ErrorInvalidDefinition
	}

	return &BaseAction{
		Command:            cmd,
		DiagnosticsManager: diagsManager,
		Parser:             parser,
		ProfilePath:        profilePath,
		Logger:             logger,
	}, nil
}

// Run is responsible for executing the base action, which is not yet implemented and returns an error.
func (ba *BaseAction) Run() error {
	return ge.ErrorNotYetImplemented
}
