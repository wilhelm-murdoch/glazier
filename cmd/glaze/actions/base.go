package actions

import (
	"path/filepath"

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

// ActionBase holds what the actions that read a profile share.
type ActionBase struct {
	Command            *cli.Command
	DiagnosticsManager *diagnostics.DiagnosticsManager
	Parser             *parser.Parser
	ProfilePath        string
	Logger             *logger.Logger
}

// NewActionBase finds and parses the profile, and returns the diagnostics as an error when the profile has syntax errors.
func NewActionBase(cmd *cli.Command, logLevel string) (*ActionBase, error) {
	profilePath, err := files.ResolveProfilePath(cmd.String("profile-path"))
	if err != nil {
		return nil, err
	}

	src, parserDiags := parser.ReadProfile(profilePath)
	if parserDiags.HasErrors() {
		return nil, diagnostics.New(profilePath, nil).Report(parserDiags)
	}

	// A file with a syntax error has no parsed form, so the source alone lets the diagnostic show the line.
	parser, parserDiags := parser.NewFromBytes(src, profilePath)
	if parserDiags.HasErrors() {
		return nil, diagnostics.New(profilePath, &hcl.File{Bytes: src}).Report(parserDiags)
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

	profile, decodeDiags := ba.Parser.Decode(spec.Session(ba.profileDir()), ctx)

	return profile, diags.Extend(decodeDiags)
}

// loadProfile decodes the profile, and writes the diagnostics when there is an error.
func (ba *ActionBase) loadProfile() (*decoders.Session, error) {
	profile, diags := ba.decodeProfile()
	if diags.HasErrors() {
		return nil, ba.DiagnosticsManager.Report(diags)
	}

	if err := profile.ResolveDirectories(ba.profileDir()); err != nil {
		return nil, err
	}

	return profile, nil
}

// profileDir returns the absolute directory of the profile, which relative starting directories use as their base directory.
func (ba *ActionBase) profileDir() string {
	dir, err := filepath.Abs(filepath.Dir(ba.ProfilePath))
	if err != nil {
		return filepath.Dir(ba.ProfilePath)
	}

	return dir
}
