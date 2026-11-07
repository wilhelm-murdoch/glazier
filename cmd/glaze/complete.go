package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/wilhelm-murdoch/glazier/pkg/files"
)

// completeProfileNames prints the names of the profile registry for a Tab after up, down, diff or format, and leaves the flags to the library.
// It never logs and never fails: zsh shows the stderr of the binary during completion, and a failing completer breaks every Tab.
func completeProfileNames(ctx context.Context, cmd *cli.Command) {
	words := cmd.Args().Slice()
	if n := len(words); n > 0 && strings.HasPrefix(words[n-1], "-") {
		cli.DefaultCompleteWithFlags(ctx, cmd)
		return
	}

	// With a name, a path or a session already given there is nothing to add, and the shells fall back to files.
	if len(words) > 0 || cmd.String("profile-path") != "" || cmd.String("session") != "" {
		return
	}

	dir, err := files.RegistryDir()
	if err != nil {
		return
	}

	profiles, _, err := files.ListProfiles(dir)
	if err != nil {
		return
	}

	// The scripts send no partial word for a name and filter by prefix themselves, so every name is printed.
	for _, profile := range profiles {
		_, _ = fmt.Fprintln(cmd.Root().Writer, profile.Name)
	}
}
