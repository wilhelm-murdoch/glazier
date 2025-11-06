package actions

import (
	"log/slog"

	"github.com/urfave/cli/v2"

	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	ge "github.com/wilhelm-murdoch/glazier/internal/errors" // ge = "Glaze Errors"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/profile"
)

type BaseAction struct {
	Context            *cli.Context
	DiagnosticsManager *diagnostics.DiagnosticsManager
	Parser             *parser.Parser
	ProfilePath        string
	Logger             *slog.Logger
}

// NewBaseAction is responsible for creating a new BaseAction instance, resolving the profile path, and initializing the diagnostics manager and parser.
func NewBaseAction(ctx *cli.Context, logger *slog.Logger) (*BaseAction, error) {
	profilePath, err := profile.ResolveProfilePath(ctx.String("profile-path"))
	if err != nil {
		return nil, err
	}

	diagsManager := diagnostics.NewDiagnosticsManager(profilePath)
	if diagsManager.HasErrors() {
		diagsManager.Write()
		return nil, ge.ErrorInvalidDefinition
	}

	parser, parserDiags := parser.NewParser(profilePath)
	if parserDiags.HasErrors() {
		diagsManager.Write()
		return nil, ge.ErrorInvalidDefinition
	}

	return &BaseAction{
		Context:            ctx,
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
