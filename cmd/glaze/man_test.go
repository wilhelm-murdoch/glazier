package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManPageNamesEveryCommandAndFlag keeps packaging/glaze.1 in step with the command tree.
// The page is hand-written, so a new command or flag must be added there by hand.
func TestManPageNamesEveryCommandAndFlag(t *testing.T) {
	page, err := os.ReadFile(filepath.Join("..", "..", "packaging", "glaze.1"))
	require.NoError(t, err)

	// roff escapes each hyphen, so the page spells --var as \-\-var.
	text := strings.ReplaceAll(string(page), `\-`, "-")

	// A plain Contains would print the whole page on a failure, so each check names only what is missing.
	names := func(label, name string) {
		assert.True(t, strings.Contains(text, name), "%s `%s` is missing from packaging/glaze.1", label, name)
	}

	app := newApp()
	for _, flag := range app.Flags {
		names("global flag", "--"+flag.Names()[0])
	}

	for _, cmd := range app.Commands {
		names("command", ".SS "+cmd.Name)
		for _, flag := range cmd.Flags {
			names("flag of "+cmd.Name, "--"+flag.Names()[0])
		}
	}

	// The library adds the completion command when the app runs, so its shells are listed here.
	for _, shell := range []string{"bash", "zsh", "fish", "pwsh"} {
		names("shell of completion", shell)
	}
}
