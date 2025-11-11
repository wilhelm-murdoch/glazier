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

// ActionBase is a type that will be ultimately embedded within other action types in
// an effort to deduplicate common fields and methods.
type ActionBase struct {
	Context            context.Context
	Command            *cli.Command
	DiagnosticsManager *diagnostics.DiagnosticsManager
	Parser             *parser.Parser
	ProfilePath        string
	Logger             *logger.Logger
}

// NewActionBase is responsible for creating a new ActionBase instance, resolving the profile path, and initializing the diagnostics manager and parser.
func NewActionBase(
	cmd *cli.Command,
	log *logger.Logger,
) (*ActionBase, error) {
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

	return &ActionBase{
		Command:            cmd,
		DiagnosticsManager: diagsManager,
		Parser:             parser,
		ProfilePath:        profilePath,
		Logger:             log,
	}, nil
}

// Run is responsible for executing the base action, which is not yet implemented and returns an error.
func (ba *ActionBase) Run() error {
	return ge.ErrorNotYetImplemented
}
