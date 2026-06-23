package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
	session, diags := p.Decode(spec.Session, ctx)
	return session, diags.HasErrors()
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

  window {
    name   = "editor"
    layout = "main-vertical"
    focus  = true

    pane {
      name     = "shell"
      focus    = true
      commands = ["vim", "ls"]
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

	assert.Equal(t, 1, len(session.Windows))
	window := session.Windows[0]
	assert.Equal(t, "editor", window.Name)
	assert.Equal(t, enums.LayoutMainVertical, window.Layout)
	assert.True(t, window.Focus)

	assert.Equal(t, 2, len(window.Panes))

	shell := window.Panes[0]
	assert.Equal(t, "shell", shell.Name)
	assert.True(t, shell.Focus)
	assert.Equal(t, []string{"vim", "ls"}, shell.Commands)
	assert.True(t, shell.Size.Valid())
	assert.Equal(t, "50%", shell.Size.X)
	assert.Equal(t, "100", shell.Size.Y)

	logs := window.Panes[1]
	assert.Equal(t, "logs", logs.Name)
	assert.Equal(t, []string{"tail -f log"}, logs.Commands)
}

func TestDecodeDefaults(t *testing.T) {
	content := `
session {
  window {
    pane {
      commands = ["echo hello"]
    }
  }
}`

	session, hasErr := decode(t, content)
	assert.False(t, hasErr)
	assert.NotNil(t, session)

	// name falls back to the default element name.
	assert.Equal(t, decoders.DefaultGlazeElementName, session.Name)
	// starting_directory falls back to the working directory.
	assert.NotEmpty(t, session.StartingDirectory)

	window := session.Windows[0]
	// layout defaults to tiled when omitted.
	assert.Equal(t, enums.LayoutTiled, window.Layout)
	assert.False(t, window.Focus)
}

func TestDecodeInvalidLayout(t *testing.T) {
	content := `
session {
  name = "demo"
  window {
    name   = "w"
    layout = "not-a-layout"
    pane {
      name     = "p"
      commands = ["echo"]
    }
  }
}`

	_, hasErr := decode(t, content)
	assert.True(t, hasErr)
}

func TestDecodeRawLayoutString(t *testing.T) {
	content := `
session {
  name = "demo"
  window {
    name   = "w"
    layout = "bb62,80x24,0,0"
    pane {
      name     = "p"
      commands = ["echo"]
    }
  }
}`

	session, hasErr := decode(t, content)
	assert.False(t, hasErr)
	window := session.Windows[0]
	assert.Equal(t, enums.LayoutUnknown, window.Layout)
	assert.Equal(t, "bb62,80x24,0,0", window.LayoutValue())
}

func TestDecodeMissingWindow(t *testing.T) {
	// window has MinItems: 1, so a session with no window is invalid.
	_, hasErr := decode(t, `session { name = "demo" }`)
	assert.True(t, hasErr)
}

func TestDecodeRejectsBlockLabels(t *testing.T) {
	// Blocks take no labels; names are supplied via the `name` attribute.
	_, hasErr := decode(t, `session "demo" { window { pane { commands = ["x"] } } }`)
	assert.True(t, hasErr)
}

func TestDecodeRejectsDuplicateSession(t *testing.T) {
	// Exactly one session block is allowed per profile. The duplicate is caught
	// before the body is decoded, so empty blocks isolate the check.
	_, hasErr := decode(t, "session {}\nsession {}")
	assert.True(t, hasErr)
}

func TestDecodeRejectsStrayTopLevel(t *testing.T) {
	// Only session and variable blocks live at the root; anything else is a
	// typo and must be rejected rather than silently ignored.
	t.Run("stray attribute", func(t *testing.T) {
		_, hasErr := decode(t, "oops = \"typo\"\nsession {}")
		assert.True(t, hasErr)
	})

	t.Run("unknown block", func(t *testing.T) {
		_, hasErr := decode(t, "windo {}\nsession {}")
		assert.True(t, hasErr)
	})
}

func TestDecodeWithVariableBlockTolerated(t *testing.T) {
	// A variable block sitting alongside the session must not be mistaken for
	// an unexpected top-level block.
	content := `
variable "x" {
  type    = string
  default = "y"
}

session {
  name = "demo"
  window {
    pane {}
  }
}`
	session, hasErr := decode(t, content)
	assert.False(t, hasErr)
	assert.Equal(t, "demo", session.Name)
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
		assert.True(t, pane.Size.Valid())
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

	t.Run("rejects an empty size block", func(t *testing.T) {
		content := `
session {
  window {
    pane {
      commands = ["echo"]
      size {}
    }
  }
}`
		_, hasErr := decode(t, content)
		assert.True(t, hasErr)
	})

	t.Run("rejects an invalid size value", func(t *testing.T) {
		content := `
session {
  window {
    pane {
      commands = ["echo"]
      size {
        x = "not-a-size"
      }
    }
  }
}`
		_, hasErr := decode(t, content)
		assert.True(t, hasErr)
	})
}

func TestDecodePaneWithoutCommands(t *testing.T) {
	// A pane with no commands attribute must not panic (commands is null).
	content := `
session {
  window {
    pane {
      name = "p"
    }
  }
}`
	session, hasErr := decode(t, content)
	assert.False(t, hasErr)
	assert.Empty(t, session.Windows[0].Panes[0].Commands)
}

func TestDecodeSessionCommands(t *testing.T) {
	content := `
session {
  name     = "demo"
  commands = ["echo booting", "tmux source ~/.tmux.conf"]
  window {
    pane {
      name = "p"
    }
  }
}`
	session, hasErr := decode(t, content)
	assert.False(t, hasErr)
	assert.Equal(t, []string{"echo booting", "tmux source ~/.tmux.conf"}, session.Commands)
}

func TestDecodeOptions(t *testing.T) {
	content := `
session {
  name = "demo"
  options = {
    "base-index" = "1"
  }
  window {
    name = "w"
    options = {
      "automatic-rename" = "off"
    }
    pane {
      name = "p"
      options = {
        "remain-on-exit" = "on"
      }
      commands = ["echo"]
    }
  }
}`
	session, hasErr := decode(t, content)
	assert.False(t, hasErr)
	assert.Equal(t, "1", session.Options["base-index"])

	window := session.Windows[0]
	assert.Equal(t, "off", window.Options["automatic-rename"])
	assert.Equal(t, "on", window.Panes[0].Options["remain-on-exit"])
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

	t.Run("rejects an invalid direction", func(t *testing.T) {
		content := `
session {
  window {
    pane {
      commands = ["echo"]
      adjust {
        direction = "sideways"
        amount    = "5"
      }
    }
  }
}`
		_, hasErr := decode(t, content)
		assert.True(t, hasErr)
	})
}
