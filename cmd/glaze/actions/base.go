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

	// ProfileName is the registry name that selected the profile, or "" for a path.
	ProfileName string

	// evalContext is the context of the last variable resolution, which the registry name check evaluates the name with.
	evalContext *hcl.EvalContext
}

// NewActionBase finds and parses the profile, and returns the diagnostics as an error when the profile has syntax errors.
func NewActionBase(cmd *cli.Command, logLevel string) (*ActionBase, error) {
	// The warning comes before the resolve, so a miss caused by the ignored variable shows it too.
	log := newLogger(cmd, logLevel)
	if profileName(cmd) != "" {
		warnIgnoredConfigHome(log)
	}

	profilePath, err := resolveProfile(cmd)
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
		Logger:             log,
		ProfileName:        profileName(cmd),
	}, nil
}

// checkRegistryName reports the diagnostic for a registry profile without a session name, and nil for a path or a named profile.
func (ba *ActionBase) checkRegistryName() error {
	if diags := ba.Parser.CheckRegistryName(ba.ProfileName, ba.evalContext); diags.HasErrors() {
		return ba.DiagnosticsManager.Report(diags)
	}

	return nil
}

// warnIgnoredConfigHome says when a relative XDG_CONFIG_HOME was ignored, so a user who set it does not look in the wrong place.
func warnIgnoredConfigHome(log *logger.Logger) {
	if value, ok := files.IgnoredConfigHome(); ok {
		log.Warn("XDG_CONFIG_HOME is not absolute, so the profile registry is ~/.config/glaze", "XDG_CONFIG_HOME", value)
	}
}

// profileName returns the positional profile name, or "" without one or for a command that the library has not run, such as one built by a test.
func profileName(cmd *cli.Command) string {
	if args := cmd.Args(); args != nil {
		return args.First()
	}

	return ""
}

// resolveProfile finds the profile of cmd: the registry profile of a positional name, or else --profile-path, ./.glaze or GLAZE_PATH.
// NewActionBase and the in-place check of format both use it, so they open the same file.
func resolveProfile(cmd *cli.Command) (string, error) {
	return files.ResolveProfilePath(cmd.String("profile-path"), profileName(cmd))
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

	ba.evalContext = ctx
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
