package actions

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/diagnostics"
	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/spec"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// ActionBase is a type that will be ultimately embedded within other action types in
// an effort to deduplicate common fields and methods.
type ActionBase struct {
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
		return nil, diagnostics.New(profilePath, nil).Report(parserDiags)
	}

	return &ActionBase{
		Command:            cmd,
		DiagnosticsManager: diagnostics.New(profilePath, parser.File),
		Parser:             parser,
		ProfilePath:        profilePath,
		Logger:             newLogger(cmd, logLevel),
	}, nil
}

// newLogger returns the logger at logLevel, or at debug level when --debug is set.
func newLogger(cmd *cli.Command, logLevel string) *logger.Logger {
	level := logger.FriendlyToInternal[logLevel]
	if cmd.Bool("debug") && level > logger.LevelDebug {
		level = logger.LevelDebug
	}

	return logger.New(level)
}

// newTmuxClient returns a client for the tmux server on --socket-path or --socket-name.
func newTmuxClient(cmd *cli.Command, log *logger.Logger) (tmux.Client, error) {
	return tmux.NewClient(cmd.String("socket-path"), cmd.String("socket-name"), log.Logger)
}

// decodeProfile resolves every declared variable and decodes the profile, returning all diagnostics.
// Every variable must resolve, because `up` builds the whole session and an unresolved one would fail mid-build.
func (ba *ActionBase) decodeProfile() (*decoders.Session, hcl.Diagnostics) {
	ctx, diags := ba.Parser.VariableContext(ba.Command.StringSlice("var"), ba.Command.String("var-file"), true)
	if diags.HasErrors() {
		return nil, diags
	}

	profile, decodeDiags := ba.Parser.Decode(spec.Session, ctx)

	return profile, diags.Extend(decodeDiags)
}

// loadProfile decodes the profile, and writes the diagnostics when there is an error.
func (ba *ActionBase) loadProfile() (*decoders.Session, error) {
	profile, diags := ba.decodeProfile()
	if diags.HasErrors() {
		return nil, ba.DiagnosticsManager.Report(diags)
	}

	return profile, nil
}
