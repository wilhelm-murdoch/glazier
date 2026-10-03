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
	assert.NoError(t, os.WriteFile(path, output, 0o600))

	p, diags := parser.New(path)
	assert.False(t, diags.HasErrors())

	session, decodeDiags := p.Decode(spec.Session(""), parser.BuildEvalContext(map[string]cty.Value{}))
	assert.False(t, decodeDiags.HasErrors())
	assert.NotNil(t, session)

	assert.Equal(t, "demo", session.Name)
	assert.Equal(t, 1, len(session.Windows))

	window := session.Windows[0]
	assert.Equal(t, "editor", window.Name)
	assert.True(t, window.Focus)
	assert.Equal(t, 1, len(window.Panes))
	assert.Equal(t, "shell", window.Panes[0].Name)
	assert.True(t, window.Panes[0].Focus)
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

	// The active window and the active pane get `focus = true` and the idle pane does not. hclwrite aligns `=`, so match `= true`.
	assert.Equal(t, 2, strings.Count(output, "= true"))
	assert.NotContains(t, output, "= false")
}

// TestGenerateProfileWithoutLayoutValidates checks that a profile with no layout decodes.
// An earlier "unknown" placeholder broke `format --validate`.
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
