package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/parser"
	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

// decodeProfile decodes a profile with these contents in a temporary directory, after it creates each subdirectory in subdirs.
// It does not resolve the directories, so a test can read the declared flags first, as diff does.
func decodeProfile(t *testing.T, contents string, subdirs ...string) (*decoders.Session, string) {
	t.Helper()

	dir := t.TempDir()
	for _, sub := range subdirs {
		require.NoError(t, os.Mkdir(filepath.Join(dir, sub), 0o700))
	}

	path := filepath.Join(dir, ".glaze")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	p, diags := parser.New(path)
	require.False(t, diags.HasErrors(), diags.Error())

	profile, diags := p.Decode(spec.Session(dir), parser.BuildEvalContext(map[string]cty.Value{}))
	require.False(t, diags.HasErrors(), diags.Error())
	require.NotNil(t, profile)

	return profile, dir
}

// desiredFrom decodes a profile and builds the model that diff compares, with the declared directories read before the resolution.
func desiredFrom(t *testing.T, contents string, subdirs ...string) (desired, string) {
	t.Helper()

	profile, dir := decodeProfile(t, contents, subdirs...)
	declared := emptyDirectories(profile)
	require.NoError(t, profile.ResolveDirectories(dir))

	return desiredSession(profile, declared), dir
}

func TestDesiredSessionNames(t *testing.T) {
	t.Run("sanitizes the names as tmux stores them", func(t *testing.T) {
		want, _ := desiredFrom(t, "session {\n  name = \"a.b:c$d\\\\e\"\n\n  window {\n    name = \"w\\\\x\\ty\"\n\n    pane {\n      name = \"p\\\\q\\nr\"\n    }\n  }\n}\n")

		// A session name loses `.`, `:`, `$` and the backslash; a window or pane name loses only the backslash and control characters.
		assert.Equal(t, "a-b-c-d-e", want.Name)
		require.Len(t, want.Windows, 1)
		assert.Equal(t, "w-x-y", want.Windows[0].Name)
		require.Len(t, want.Windows[0].Panes, 1)
		assert.Equal(t, "p-q-r", want.Windows[0].Panes[0].Name)
	})

	t.Run("keeps a name that tmux accepts", func(t *testing.T) {
		want, _ := desiredFrom(t, "session {\n  name = \"api-1_2 three\"\n\n  window {\n    name = \"edit.go\"\n\n    pane {\n      name = \"shell:1\"\n    }\n  }\n}\n")

		assert.Equal(t, "api-1_2 three", want.Name)
		assert.Equal(t, "edit.go", want.Windows[0].Name)
		assert.Equal(t, "shell:1", want.Windows[0].Panes[0].Name)
	})

	t.Run("names an absent name default at every level", func(t *testing.T) {
		want, _ := desiredFrom(t, "session {\n  window {\n    pane {}\n  }\n}\n")

		assert.Equal(t, decoders.DefaultGlazeElementName, want.Name)
		assert.Equal(t, decoders.DefaultGlazeElementName, want.Windows[0].Name)
		assert.Equal(t, decoders.DefaultGlazeElementName, want.Windows[0].Panes[0].Name)
	})
}

func TestDesiredSessionFocus(t *testing.T) {
	t.Run("is -1 when the profile declares none", func(t *testing.T) {
		want, _ := desiredFrom(t, validProfile)

		assert.Equal(t, -1, want.Focus)
		assert.Equal(t, -1, want.Windows[0].Focus)
	})

	t.Run("takes the last focused window and the last focused pane of each window", func(t *testing.T) {
		want, _ := desiredFrom(t, `session {
  name = "demo"

  window {
    name  = "one"
    focus = true

    pane {
      name  = "a"
      focus = true
    }

    pane {
      name  = "b"
      focus = true
    }

    pane {
      name = "c"
    }
  }

  window {
    name  = "two"
    focus = true

    pane {
      name = "d"
    }
  }

  window {
    name = "three"

    pane {
      name  = "e"
      focus = true
    }
  }
}
`)

		// up selects each focused item in file order, so the last one is the one that stays selected.
		assert.Equal(t, 1, want.Focus)
		assert.Equal(t, 1, want.Windows[0].Focus)
		assert.Equal(t, -1, want.Windows[1].Focus)
		assert.Equal(t, 0, want.Windows[2].Focus)
	})
}

func TestDesiredSessionDirectories(t *testing.T) {
	t.Run("marks every directory as inherited when the profile declares none", func(t *testing.T) {
		want, _ := desiredFrom(t, validProfile)

		pwd, err := os.Getwd()
		require.NoError(t, err)

		assert.False(t, want.DirectoryDeclared)
		assert.Equal(t, pwd, want.Directory)
		assert.True(t, want.Windows[0].Panes[0].InheritsSessionDirectory)
		assert.Equal(t, want.Directory, want.Windows[0].Panes[0].Directory)
	})

	t.Run("resolves a declared session directory and nothing inherits the current directory", func(t *testing.T) {
		want, dir := desiredFrom(t, registryProfile)

		assert.True(t, want.DirectoryDeclared)
		assert.Equal(t, dir, want.Directory)
		assert.False(t, want.Windows[0].Panes[0].InheritsSessionDirectory)
		assert.Equal(t, dir, want.Windows[0].Panes[0].Directory)
	})

	t.Run("a declared window directory stops the inheritance of its panes", func(t *testing.T) {
		want, dir := desiredFrom(t, `session {
  name = "demo"

  window {
    name               = "declared"
    starting_directory = "sub"

    pane {
      name = "a"
    }
  }

  window {
    name = "plain"

    pane {
      name = "b"
    }

    pane {
      name               = "c"
      starting_directory = "other"
    }
  }
}
`, "sub", "other")

		assert.False(t, want.DirectoryDeclared)
		assert.False(t, want.Windows[0].Panes[0].InheritsSessionDirectory)
		assert.Equal(t, filepath.Join(dir, "sub"), want.Windows[0].Panes[0].Directory)

		// Only the pane with no directory of its own and no window directory inherits the session directory.
		assert.True(t, want.Windows[1].Panes[0].InheritsSessionDirectory)
		assert.False(t, want.Windows[1].Panes[1].InheritsSessionDirectory)
		assert.Equal(t, filepath.Join(dir, "other"), want.Windows[1].Panes[1].Directory)
	})
}

func TestDesiredSessionAutomaticRename(t *testing.T) {
	cases := []struct {
		name    string
		window  string
		session string
		want    bool
	}{
		{"on in the window", "on", "", true},
		{"1 in the window", "1", "", true},
		{"yes in the window", "yes", "", true},
		{"On in the window", "On", "", true},
		{"off in the window", "off", "", false},
		{"nothing anywhere", "", "", false},
		{"on in the session block", "", "on", true},
		{"1 in the session block", "", "1", true},
		{"off in the session block", "", "off", false},
		{"off in the window wins over on in the session block", "off", "on", false},
		{"on in the window wins over off in the session block", "on", "off", true},
	}

	option := func(value string) string {
		if value == "" {
			return ""
		}

		return fmt.Sprintf("  options = {\n    \"automatic-rename\" = %q\n  }\n", value)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			profile := "session {\n  name = \"demo\"\n" + option(c.session) + "\n  window {\n    name = \"main\"\n  " + option(c.window) + "\n    pane {\n      name = \"shell\"\n    }\n  }\n}\n"
			want, _ := desiredFrom(t, profile)

			assert.Equal(t, c.want, want.Windows[0].AutomaticDeclared)
		})
	}
}

func TestDesiredSessionDropsTheLayout(t *testing.T) {
	tiled, _ := desiredFrom(t, validProfile)
	raw, _ := desiredFrom(t, "session {\n  name = \"demo\"\n\n  window {\n    name   = \"main\"\n    layout = \"bb62,80x24,0,0\"\n\n    pane {\n      name = \"shell\"\n    }\n  }\n}\n")

	// The two profiles differ only in the layout, which diff does not compare.
	tiled.Directory, raw.Directory = "", ""
	tiled.Windows[0].Panes[0].Directory, raw.Windows[0].Panes[0].Directory = "", ""
	assert.Equal(t, tiled, raw)
}

func TestEmptyDirectories(t *testing.T) {
	profile, _ := decodeProfile(t, `session {
  name               = "demo"
  starting_directory = "."

  window {
    name = "one"

    pane {
      name               = "a"
      starting_directory = "sub"
    }

    pane {
      name = "b"
    }
  }

  window {
    name               = "two"
    starting_directory = "sub"

    pane {
      name = "c"
    }
  }
}
`, "sub")

	flags := emptyDirectories(profile)

	assert.True(t, flags.session)
	require.Len(t, flags.windows, 2)
	assert.False(t, flags.windows[0].window)
	assert.Equal(t, []bool{true, false}, flags.windows[0].panes)
	assert.True(t, flags.windows[1].window)
	assert.Equal(t, []bool{false}, flags.windows[1].panes)
}
