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
// The page is hand-written, so a new command, flag, name or variable must be added there by hand.
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

	// The SYNOPSIS shows [ name ] exactly for the commands that take a profile name, so save keeps its line as it is.
	synopsis := section(text, "SYNOPSIS")
	for _, cmd := range app.Commands {
		entry := synopsisEntry(synopsis, cmd.Name)
		assert.NotEmpty(t, entry, "command `%s` is missing from the SYNOPSIS of packaging/glaze.1", cmd.Name)
		assert.Equal(t, cmd.ArgsUsage != "", strings.Contains(entry, ".RI [ name ]"), "the SYNOPSIS of `%s` must show [ name ] only when the command takes a name", cmd.Name)
	}

	environment := section(text, "ENVIRONMENT")
	for _, variable := range []string{"GLAZE_REGISTRY", "XDG_CONFIG_HOME", "GLAZE_PATH", "NO_COLOR", "TMUX"} {
		assert.True(t, strings.Contains(environment, ".B "+variable+"\n"), "environment variable `%s` is missing from packaging/glaze.1", variable)
	}

	// The library adds the completion command when the app runs, so its shells are listed here.
	for _, shell := range []string{"bash", "zsh", "fish", "pwsh"} {
		names("shell of completion", shell)
	}
}

// section returns the text of the .SH section name up to the next .SH, or "" when the page has no such section.
func section(text, name string) string {
	_, rest, found := strings.Cut(text, ".SH "+name+"\n")
	if !found {
		return ""
	}

	if end := strings.Index(rest, "\n.SH "); end >= 0 {
		return rest[:end]
	}

	return rest
}

// synopsisEntry returns the lines of the SYNOPSIS for `glaze <command>` from its .B line to the next .br, or "" when there is none.
func synopsisEntry(synopsis, command string) string {
	_, rest, found := strings.Cut(synopsis, ".B glaze "+command+"\n")
	if !found {
		return ""
	}

	if end := strings.Index(rest, "\n.br"); end >= 0 {
		return rest[:end]
	}

	return rest
}
