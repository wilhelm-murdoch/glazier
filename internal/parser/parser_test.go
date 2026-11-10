package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/spec"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// writeGlaze writes the given HCL content to a temp .glaze file and returns its path.
func writeGlaze(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.glaze")
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// parse parses src in memory and stops the test when the parse fails.
func parse(t *testing.T, src string) *Parser {
	t.Helper()
	p, diags := NewFromBytes([]byte(src), "test.glaze")
	require.False(t, diags.HasErrors(), diags.Error())
	return p
}

func TestNew(t *testing.T) {
	t.Run("parses a valid file", func(t *testing.T) {
		path := writeGlaze(t, `session { name = "demo" }`)
		p, diags := New(path)
		assert.False(t, diags.HasErrors())
		assert.NotNil(t, p)
		assert.NotNil(t, p.File)
	})

	t.Run("returns diagnostics for a syntax error", func(t *testing.T) {
		path := writeGlaze(t, `session {`)
		p, diags := New(path)
		assert.True(t, diags.HasErrors())
		assert.Nil(t, p)
	})

	t.Run("returns diagnostics for a missing file", func(t *testing.T) {
		p, diags := New(filepath.Join(t.TempDir(), "missing.glaze"))
		assert.True(t, diags.HasErrors())
		assert.Nil(t, p)
	})
}

// decode parses content, builds an eval context, and decodes against the
// session spec, returning the decoded session and whether decoding errored.
func decode(t *testing.T, content string) (*decoders.Session, bool) {
	t.Helper()
	path := writeGlaze(t, content)
	p, diags := New(path)
	if diags.HasErrors() {
		return nil, true
	}

	ctx := BuildEvalContext(map[string]cty.Value{})
	session, diags := p.Decode(spec.Session(""), ctx)
	return session, diags.HasErrors()
}

func TestDecodeWarnsAboutWindowAndPaneNames(t *testing.T) {
	path := writeGlaze(t, "session {\n  name = \"demo\"\n  window {\n    name = \"w\\\\z\"\n    pane {\n      name = \"p\\\\z\"\n    }\n  }\n}\n")
	p, diags := New(path)
	assert.False(t, diags.HasErrors())

	_, diags = p.Decode(spec.Session(""), BuildEvalContext(map[string]cty.Value{}))
	assert.False(t, diags.HasErrors())

	var summaries []string
	for _, d := range diags {
		summaries = append(summaries, d.Summary)
	}

	assert.ElementsMatch(t, []string{"Window name will be changed", "Pane name will be changed"}, summaries)
}

func TestDecodeReturnsWarnings(t *testing.T) {
	path := writeGlaze(t, "session {\n  name = \"a.b\"\n  window {\n    pane {}\n  }\n}\n")
	p, diags := New(path)
	assert.False(t, diags.HasErrors())

	session, diags := p.Decode(spec.Session(""), BuildEvalContext(map[string]cty.Value{}))
	assert.NotNil(t, session)
	assert.False(t, diags.HasErrors())
	if assert.Len(t, diags, 1) {
		assert.Equal(t, "Session name will be changed", diags[0].Summary)
		if assert.NotNil(t, diags[0].Subject) {
			assert.Equal(t, 2, diags[0].Subject.Start.Line)
		}
	}
}

func TestDecodeFullProfile(t *testing.T) {
	dir := t.TempDir()
	content := `
session {
  name               = "my-session"
  starting_directory = "` + dir + `"
  envs = {
    EDITOR = "vim"
  }
  hooks = {
    "session-created" = "echo hi"
  }
  commands = ["echo session"]
  options = {
    "base-index" = "1"
  }

  window {
    name   = "editor"
    layout = "main-vertical"
    focus  = true
    options = {
      "automatic-rename" = "off"
    }

    pane {
      name     = "shell"
      focus    = true
      commands = ["vim", "ls"]
      options = {
        "remain-on-exit" = "on"
      }
      size {
        x = "50%"
        y = "100"
      }
    }

    pane {
      name     = "logs"
      commands = ["tail -f log"]
    }
  }
}`

	session, hasErr := decode(t, content)
	assert.False(t, hasErr)
	assert.NotNil(t, session)

	assert.Equal(t, "my-session", session.Name)
	assert.Equal(t, dir, session.StartingDirectory)
	assert.Equal(t, "vim", session.Envs["EDITOR"])
	assert.Equal(t, "echo hi", session.Hooks["session-created"])
	assert.Equal(t, []string{"echo session"}, session.Commands)
	assert.Equal(t, "1", session.Options["base-index"])

	assert.Equal(t, 1, len(session.Windows))
	window := session.Windows[0]
	assert.Equal(t, "editor", window.Name)
	assert.Equal(t, enums.LayoutMainVertical, window.Layout)
	assert.True(t, window.Focus)
	assert.Equal(t, "off", window.Options["automatic-rename"])

	assert.Equal(t, 2, len(window.Panes))

	shell := window.Panes[0]
	assert.Equal(t, "shell", shell.Name)
	assert.True(t, shell.Focus)
	assert.Equal(t, []string{"vim", "ls"}, shell.Commands)
	assert.True(t, shell.Size.IsSet())
	assert.Equal(t, "50%", shell.Size.X)
	assert.Equal(t, "100", shell.Size.Y)
	assert.Equal(t, "on", shell.Options["remain-on-exit"])

	logs := window.Panes[1]
	assert.Equal(t, "logs", logs.Name)
	assert.Equal(t, []string{"tail -f log"}, logs.Commands)
}

func TestDecodeDefaults(t *testing.T) {
	content := `
session {
  window {
    pane {}
  }
}`

	session, hasErr := decode(t, content)
	assert.False(t, hasErr)
	assert.NotNil(t, session)

	// name falls back to the default element name.
	assert.Equal(t, decoders.DefaultGlazeElementName, session.Name)
	// starting_directory stays empty, so that ResolveDirectories can tell an unset directory from a set one.
	assert.Empty(t, session.StartingDirectory)
	assert.Empty(t, session.Windows[0].StartingDirectory)
	assert.Empty(t, session.Windows[0].Panes[0].StartingDirectory)
	// The decode gives a pane without a commands attribute a null list, and it must not panic.
	assert.Empty(t, session.Windows[0].Panes[0].Commands)

	window := session.Windows[0]
	// layout defaults to tiled when omitted.
	assert.Equal(t, enums.LayoutTiled, window.Layout)
	assert.False(t, window.Focus)
}

// Each row is a profile structure that the decode must reject. The root takes only session and variable blocks, so the decode rejects a typo.
func TestDecodeRejectsInvalidStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"a session without a window", `session { name = "demo" }`},                  // window has MinItems 1
		{"a block label", `session "demo" { window { pane { commands = ["x"] } } }`}, // names come from the name attribute
		{"a second session block", "session {}\nsession {}"},                         // the duplicate is caught before the body is decoded, so empty blocks isolate the check
		{"a stray top-level attribute", "oops = \"typo\"\nsession {}"},
		{"an unknown top-level block", "windo {}\nsession {}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, hasErr := decode(t, tc.src)
			assert.True(t, hasErr)
		})
	}
}

func TestDecodeSizeBlock(t *testing.T) {
	t.Run("accepts a valid size block", func(t *testing.T) {
		content := `
session {
  window {
    pane {
      commands = ["echo"]
      size {
        x = "80"
        y = "50%"
      }
    }
  }
}`
		session, hasErr := decode(t, content)
		assert.False(t, hasErr)
		pane := session.Windows[0].Panes[0]
		assert.True(t, pane.Size.IsSet())
		assert.Equal(t, "80", pane.Size.X)
		assert.Equal(t, "50%", pane.Size.Y)
	})

	t.Run("accepts a size block with only one dimension", func(t *testing.T) {
		content := `
session {
  window {
    pane {
      commands = ["echo"]
      size {
        x = "80"
      }
    }
  }
}`
		session, hasErr := decode(t, content)
		assert.False(t, hasErr)
		pane := session.Windows[0].Panes[0]
		assert.Equal(t, "80", pane.Size.X)
		assert.Equal(t, "", pane.Size.Y)
	})
}

func TestDecodeAdjust(t *testing.T) {
	t.Run("decodes adjust blocks in order", func(t *testing.T) {
		content := `
session {
  window {
    pane {
      commands = ["echo"]
      adjust {
        direction = "up"
        amount    = "5"
      }
      adjust {
        direction = "right"
        amount    = "10"
      }
    }
  }
}`
		session, hasErr := decode(t, content)
		assert.False(t, hasErr)

		pane := session.Windows[0].Panes[0]
		assert.Len(t, pane.Adjustments, 2)
		assert.Equal(t, enums.AdjustmentUp, pane.Adjustments[0].Direction)
		assert.Equal(t, "5", pane.Adjustments[0].Amount)
		assert.Equal(t, enums.AdjustmentRight, pane.Adjustments[1].Direction)
		assert.Equal(t, "10", pane.Adjustments[1].Amount)
	})
}
