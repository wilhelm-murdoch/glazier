package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

func TestGenerateProfile(t *testing.T) {
	captured := savedSession{
		Name:              "demo",
		StartingDirectory: "/home/user",
		Windows: []savedWindow{
			{
				Name:   "editor",
				Layout: "main-vertical",
				Panes: []savedPane{
					{Name: "shell", StartingDirectory: "/home/user/project"},
					{Name: "logs", StartingDirectory: "/var/log"},
				},
			},
			{
				Name:   "scratch",
				Layout: "tiled",
				Panes: []savedPane{
					{Name: "repl"},
				},
			},
		},
	}

	output := string(generateProfile(captured))

	assert.Contains(t, output, `"demo"`)
	assert.Contains(t, output, `"/home/user"`)
	assert.Contains(t, output, `"main-vertical"`)
	assert.Contains(t, output, `"shell"`)
	assert.Contains(t, output, `"repl"`)
	assert.Contains(t, output, "session {")
	assert.Contains(t, output, "window {")
	assert.Contains(t, output, "pane {")
}

// TestGenerateProfileRoundTrips proves the generated output is a valid glaze
// definition by parsing and decoding it back through the real spec.
func TestGenerateProfileRoundTrips(t *testing.T) {
	captured := savedSession{
		Name:              "demo",
		StartingDirectory: t.TempDir(),
		Windows: []savedWindow{
			{
				Name:   "editor",
				Layout: "main-vertical",
				Focus:  true,
				Panes: []savedPane{
					{Name: "shell", StartingDirectory: t.TempDir(), Focus: true},
				},
			},
		},
	}

	output := generateProfile(captured)

	path := filepath.Join(t.TempDir(), "saved.glaze")
	assert.NoError(t, os.WriteFile(path, output, 0o644))

	p, diags := parser.New(path)
	assert.False(t, diags.HasErrors())

	session, decodeDiags := p.Decode(spec.Session, parser.BuildEvalContext(map[string]cty.Value{}))
	assert.False(t, decodeDiags.HasErrors())
	assert.NotNil(t, session)

	assert.Equal(t, "demo", session.Name)
	assert.Equal(t, 1, session.Windows.Length())

	window := session.Windows.Items()[0]
	assert.Equal(t, "editor", window.Name)
	assert.True(t, window.Focus)
	assert.Equal(t, 1, window.Panes.Length())
	assert.Equal(t, "shell", window.Panes.Items()[0].Name)
	assert.True(t, window.Panes.Items()[0].Focus)
}

func TestGenerateProfileOmitsEmptyOptionals(t *testing.T) {
	captured := savedSession{
		Name: "minimal",
		Windows: []savedWindow{
			{
				Name:  "w",
				Panes: []savedPane{{Name: "p"}},
			},
		},
	}

	output := string(generateProfile(captured))

	assert.NotContains(t, output, "starting_directory")
	assert.NotContains(t, output, "layout")
	assert.NotContains(t, output, "focus")
}

func TestGenerateProfileEmitsFocus(t *testing.T) {
	captured := savedSession{
		Name: "demo",
		Windows: []savedWindow{
			{
				Name:  "focused",
				Focus: true,
				Panes: []savedPane{
					{Name: "active", Focus: true},
					{Name: "idle"},
				},
			},
		},
	}

	output := string(generateProfile(captured))

	// Both the active window and active pane should emit a `focus` attribute
	// set to true, and the idle pane should not. hclwrite aligns the `=`, so
	// match on `= true` rather than exact spacing.
	assert.Equal(t, 2, strings.Count(output, "= true"))
	assert.NotContains(t, output, "= false")
}

// TestGenerateProfileWithoutLayoutValidates guards the original failure mode:
// captureSession cannot recover a window's layout, so it omits the attribute.
// A profile with no layout must still decode cleanly (an earlier "unknown"
// placeholder broke `format --validate`).
func TestGenerateProfileWithoutLayoutValidates(t *testing.T) {
	captured := savedSession{
		Name: "demo",
		Windows: []savedWindow{
			{
				Name:  "editor",
				Panes: []savedPane{{Name: "shell"}},
			},
		},
	}

	output := generateProfile(captured)
	assert.NotContains(t, string(output), "layout")

	path := filepath.Join(t.TempDir(), "saved.glaze")
	assert.NoError(t, os.WriteFile(path, output, 0o644))

	p, diags := parser.New(path)
	assert.False(t, diags.HasErrors())

	_, decodeDiags := p.Decode(spec.Session, parser.BuildEvalContext(map[string]cty.Value{}))
	assert.False(t, decodeDiags.HasErrors())
}
