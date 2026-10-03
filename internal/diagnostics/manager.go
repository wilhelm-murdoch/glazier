package diagnostics

import (
	"cmp"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"

	"github.com/wilhelm-murdoch/glazier/internal/term"
)

const diagnosticTextWriterWidth = 78

// ErrHasDiagnostics means that the diagnostics hold an error. Write has already printed them, so the caller only stops.
var ErrHasDiagnostics = errors.New("the glaze profile contains errors")

// DiagnosticsManager embeds the structure of hcl.Diagnostics and combines it
// with a DiagnosticsWriter to simplify use.
type DiagnosticsManager struct {
	hcl.Diagnostics
	Writer hcl.DiagnosticWriter

	// profile is the file of the profile, and files maps each file that the writer can show a line of to its source.
	// The writer holds the same map, so a source added to it later also shows.
	profile string
	files   map[string]*hcl.File
}

// Extend adds diags to the manager in place, unlike hcl.Diagnostics.Extend, which returns a new slice.
func (dm *DiagnosticsManager) Extend(diags hcl.Diagnostics) {
	dm.Diagnostics = dm.Diagnostics.Extend(diags)
}

// Append appends a single diagnostic to the accumulated set. It shadows the
// embedded hcl.Diagnostics.Append for the same in-place reason as Extend.
func (dm *DiagnosticsManager) Append(diag *hcl.Diagnostic) {
	dm.Diagnostics = dm.Diagnostics.Append(diag)
}

// Write writes every diagnostic in source order and returns ErrHasDiagnostics when there is an error, so the caller stops.
func (dm *DiagnosticsManager) Write() error {
	dm.sortBySource()
	dm.loadSources()

	if writeErr := dm.Writer.WriteDiagnostics(dm.Diagnostics); writeErr != nil {
		return writeErr
	}

	if dm.HasErrors() {
		return ErrHasDiagnostics
	}

	return nil
}

// New returns a manager that writes the diagnostics of the file at filePath to stderr, so stdout carries only command output.
func New(filePath string, file *hcl.File) *DiagnosticsManager {
	files := map[string]*hcl.File{filePath: file}

	return &DiagnosticsManager{
		Diagnostics: hcl.Diagnostics{},
		Writer:      hcl.NewDiagnosticTextWriter(os.Stderr, files, diagnosticTextWriterWidth, term.ColorEnabled(os.Stderr)),
		profile:     filePath,
		files:       files,
	}
}

// sortBySource puts the diagnostics in a fixed order: first those without a place, then those of the profile,
// then those of each other file by name, each file from top to bottom. The decoder reports in a random order.
func (dm *DiagnosticsManager) sortBySource() {
	slices.SortStableFunc(dm.Diagnostics, func(a, b *hcl.Diagnostic) int {
		switch {
		case a.Subject == nil || b.Subject == nil:
			// A diagnostic without a place comes first; two of them keep their order.
			return cmp.Compare(boolRank(a.Subject != nil), boolRank(b.Subject != nil))
		case a.Subject.Filename != b.Subject.Filename:
			return cmp.Or(
				cmp.Compare(boolRank(a.Subject.Filename != dm.profile), boolRank(b.Subject.Filename != dm.profile)),
				strings.Compare(a.Subject.Filename, b.Subject.Filename),
			)
		default:
			return cmp.Compare(a.Subject.Start.Byte, b.Subject.Start.Byte)
		}
	})
}

// loadSources reads each regular file that a diagnostic points into and the writer does not know yet, for example
// a --var-file, so the writer shows the line. It skips a pipe, because a second read of a pipe can block.
func (dm *DiagnosticsManager) loadSources() {
	if dm.files == nil {
		return
	}

	for _, diag := range dm.Diagnostics {
		if diag.Subject == nil {
			continue
		}

		name := diag.Subject.Filename
		if _, known := dm.files[name]; known {
			continue
		}

		info, err := os.Stat(name)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}

		// The name comes from a diagnostic about a file that glaze already read, such as the user's --var-file.
		src, err := os.ReadFile(name) //nolint:gosec // G304
		if err != nil {
			continue
		}

		dm.files[name] = &hcl.File{Bytes: src}
	}
}

// boolRank orders false before true.
func boolRank(b bool) int {
	if b {
		return 1
	}

	return 0
}

// Report adds diags and writes every diagnostic, returning ErrHasDiagnostics when there is an error.
func (dm *DiagnosticsManager) Report(diags hcl.Diagnostics) error {
	dm.Extend(diags)
	return dm.Write()
}
