package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/diagnostics" // ge = "Glaze Errors"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/spec"
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

// NewActionBase is responsible for creating a new ActionBase struct value, resolving
// the profile path, and initializing the diagnostics manager and parser.
func NewActionBase(cmd *cli.Command, logLevel string) (*ActionBase, error) {
	profilePath, err := files.ResolveProfilePath(cmd.String("profile-path"))
	if err != nil {
		return nil, err
	}

	parser, parserDiags := parser.New(profilePath)
	if parserDiags.HasErrors() {
		diagsManager := diagnostics.New(profilePath, nil)
		diagsManager.Extend(parserDiags)
		return nil, diagsManager.Write()
	}

	diagsManager := diagnostics.New(profilePath, parser.File)
	if diagsManager.HasErrors() {
		return nil, diagsManager.Write()
	}

	log := logger.New(logger.FriendlyToInternal[logLevel])

	return &ActionBase{
		Command:            cmd,
		DiagnosticsManager: diagsManager,
		Parser:             parser,
		ProfilePath:        profilePath,
		Logger:             log,
	}, nil
}

// Run is responsible for executing the base action, which is not yet implemented
// and returns an error.
func (ba *ActionBase) Run() error {
	return errors.New("this feature is not yet implemented")
}

// loadProfile handles parsing variables and decoding the HCL definition.
func (ba *ActionBase) loadProfile() (*decoders.Session, error) {
	variables, err := parser.CollectVariables(ba.Command.StringSlice("var"))
	if err != nil {
		return nil, fmt.Errorf("could not parse specified variables: %w", err)
	}

	profile, decodeDiags := ba.Parser.Decode(
		spec.Session,
		parser.BuildEvalContext(variables),
	)
	if decodeDiags.HasErrors() {
		ba.DiagnosticsManager.Extend(decodeDiags)
		return nil, ba.DiagnosticsManager.Write()
	}

	return profile, nil
}
