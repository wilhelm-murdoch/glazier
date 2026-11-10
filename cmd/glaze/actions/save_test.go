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

// TestGenerateProfileRoundTrips proves the generated output is a valid glaze
// definition by parsing and decoding it back through the real spec.
func TestGenerateProfileRoundTrips(t *testing.T) {
	sessionDir := t.TempDir()
	paneDir := t.TempDir()
	captured := savedSession{
		Name:              "demo",
		StartingDirectory: sessionDir,
		Windows: []savedWindow{
			{
				Name:   "editor",
				Layout: "main-vertical",
				Focus:  true,
				Panes: []savedPane{
					{Name: "shell", StartingDirectory: paneDir, Focus: true},
					{Name: "logs"},
				},
			},
			{
				Name:   "scratch",
				Layout: "tiled",
				Panes:  []savedPane{{Name: "repl"}},
			},
		},
	}

	output := generateProfile(captured)

	path := filepath.Join(t.TempDir(), "saved.glaze")
	assert.NoError(t, os.WriteFile(path, output, 0o600))

	p, diags := parser.New(path)
	assert.False(t, diags.HasErrors())

	session, decodeDiags := p.Decode(spec.Session(""), parser.BuildEvalContext(map[string]cty.Value{}))
	assert.False(t, decodeDiags.HasErrors())
	assert.NotNil(t, session)

	assert.Equal(t, "demo", session.Name)
	assert.Equal(t, sessionDir, session.StartingDirectory)
	assert.Equal(t, 2, len(session.Windows))

	window := session.Windows[0]
	assert.Equal(t, "editor", window.Name)
	assert.Equal(t, "main-vertical", window.LayoutValue())
	assert.True(t, window.Focus)
	assert.Equal(t, 2, len(window.Panes))
	assert.Equal(t, "shell", window.Panes[0].Name)
	assert.Equal(t, paneDir, window.Panes[0].StartingDirectory)
	assert.True(t, window.Panes[0].Focus)
	assert.False(t, window.Panes[1].Focus)

	scratch := session.Windows[1]
	assert.Equal(t, "scratch", scratch.Name)
	assert.Equal(t, 1, len(scratch.Panes))
	assert.Equal(t, "repl", scratch.Panes[0].Name)
}

// A layout that tmux 3.9 prints is JSON, so its quotes must survive the HCL string and the profile must validate.
func TestGenerateProfileRoundTripsAJSONLayout(t *testing.T) {
	layout := `{"V":2,"L":{"t":"h","w":80,"h":24,"x":0,"y":0,"c":[{"t":"p","w":40,"h":24,"x":0,"y":0,"i":0,"I":"%0"},{"t":"p","w":39,"h":24,"x":41,"y":0,"i":1,"I":"%1"}]}}`
	captured := savedSession{
		Name:    "demo",
		Windows: []savedWindow{{Name: "w", Layout: layout, Panes: []savedPane{{Name: "a"}, {Name: "b"}}}},
	}

	path := filepath.Join(t.TempDir(), "saved.glaze")
	assert.NoError(t, os.WriteFile(path, generateProfile(captured), 0o600))

	p, diags := parser.New(path)
	assert.False(t, diags.HasErrors())

	session, decodeDiags := p.Decode(spec.Session(""), parser.BuildEvalContext(map[string]cty.Value{}))
	assert.False(t, decodeDiags.HasErrors(), decodeDiags.Error())
	assert.Equal(t, layout, session.Windows[0].LayoutValue())
}

// TestGenerateProfileOmitsEmptyOptionalsAndValidates checks that a profile with no layout, directory or focus leaves them out and decodes.
// An earlier "unknown" placeholder broke `format --validate`.
func TestGenerateProfileOmitsEmptyOptionalsAndValidates(t *testing.T) {
	captured := savedSession{
		Name: "minimal",
		Windows: []savedWindow{
			{
				Name:  "w",
				Panes: []savedPane{{Name: "p"}},
			},
		},
	}

	output := generateProfile(captured)

	assert.NotContains(t, string(output), "starting_directory")
	assert.NotContains(t, string(output), "layout")
	assert.NotContains(t, string(output), "focus")

	path := filepath.Join(t.TempDir(), "saved.glaze")
	assert.NoError(t, os.WriteFile(path, output, 0o600))

	p, diags := parser.New(path)
	assert.False(t, diags.HasErrors())

	_, decodeDiags := p.Decode(spec.Session(""), parser.BuildEvalContext(map[string]cty.Value{}))
	assert.False(t, decodeDiags.HasErrors())
}

func TestGenerateProfileLeavesOutEmptyNames(t *testing.T) {
	captured := savedSession{
		Name: "raw",
		Windows: []savedWindow{
			{Layout: "tiled", Panes: []savedPane{{}, {Name: "tail"}}},
		},
	}

	output := string(generateProfile(captured))
	assert.Equal(t, 2, strings.Count(output, "name"), "only the session and the named pane have a name:\n"+output)

	p, diags := parser.NewFromBytes([]byte(output), "saved.glaze")
	assert.False(t, diags.HasErrors(), diags.Error())

	_, diags = p.Decode(spec.Session(""), parser.BuildEvalContext(map[string]cty.Value{}))
	assert.False(t, diags.HasErrors(), diags.Error())
}
