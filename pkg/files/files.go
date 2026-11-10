package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrTildeUser means that a path starts with `~user`, which glaze does not expand.
	ErrTildeUser = errors.New("glaze expands only `~` and `~/`, not `~user`")

	// ErrProfileNotFound means that glaze cannot find the profile to read.
	ErrProfileNotFound = errors.New("glaze profile not found")
)

// FileExists reports whether path, after symlinks, exists and is not a directory.
func FileExists(path string) bool {
	// The path is the user's own --profile-path or GLAZE_PATH input to a local CLI.
	info, err := os.Stat(path) //nolint:gosec // G703

	return err == nil && !info.IsDir()
}

// ExpandPath replaces a leading `~` or `~/` with the home directory. It rejects `~user` and fails when there is no home directory.
func ExpandPath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		if strings.HasPrefix(path, "~") {
			return path, ErrTildeUser
		}

		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path, fmt.Errorf("could not expand `~`: %w", err)
	}

	return home + path[1:], nil
}

// ResolveDirectory expands `~` in path and makes a relative path absolute against baseDirectory.
func ResolveDirectory(path, baseDirectory string) (string, error) {
	path, err := ExpandPath(path)
	if err != nil {
		return path, err
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDirectory, path)
	}

	return filepath.Clean(path), nil
}

// ResolveProfilePath returns the registry profile of name, or else the profile from --profile-path, the current directory or GLAZE_PATH, in that order.
// A name and a --profile-path together are rejected before this call, so the name wins here.
func ResolveProfilePath(profilePath, name string) (string, error) {
	if name != "" {
		return resolveRegistryProfile(name)
	}

	if profilePath != "" {
		expanded, err := ExpandPath(profilePath)
		if err != nil {
			return profilePath, fmt.Errorf("%w: %w", ErrProfileNotFound, err)
		}

		if IsDir(expanded) {
			return profilePath, fmt.Errorf("%w: `%s` is a directory; --profile-path must name a profile file", ErrProfileNotFound, profilePath)
		}

		if !FileExists(expanded) {
			return profilePath, fmt.Errorf("%w: `%s` does not exist", ErrProfileNotFound, profilePath)
		}

		return expanded, nil
	}

	// Without a current directory, for example one that was deleted, the search skips it and still tries GLAZE_PATH.
	local := ".glaze"
	if cwd, err := os.Getwd(); err == nil {
		local = filepath.Join(cwd, ".glaze")

		if FileExists(local) {
			return local, nil
		}

		if IsDir(local) {
			return local, fmt.Errorf("%w: `%s` is a directory, not a profile", ErrProfileNotFound, local)
		}
	}

	if glazePath := os.Getenv("GLAZE_PATH"); glazePath != "" {
		// A GLAZE_PATH that glaze cannot expand stays as it is, so the search below fails with the usual message.
		if expanded, err := ExpandPath(glazePath); err == nil {
			glazePath = expanded
		}

		// GLAZE_PATH names the directory that holds .glaze, so a path to the file itself is a common mistake.
		if FileExists(glazePath) {
			return glazePath, fmt.Errorf("%w: GLAZE_PATH must be a directory, but `%s` is a file; set GLAZE_PATH to `%s`",
				ErrProfileNotFound, glazePath, filepath.Dir(glazePath))
		}

		profile := filepath.Join(glazePath, ".glaze")
		if FileExists(profile) {
			return profile, nil
		}

		if IsDir(profile) {
			return profile, fmt.Errorf("%w: `%s` is a directory, not a profile", ErrProfileNotFound, profile)
		}
	}

	return local, fmt.Errorf(
		"%w:\n - tried using --profile-path\n - searching the current directory\n - looking up GLAZE_PATH environment variable\n - no profile name was given; `glaze up <name>` reads the profile registry and `glaze ls --profiles` lists it",
		ErrProfileNotFound,
	)
}

// IsDir reports whether path, after symlinks, is a directory.
func IsDir(path string) bool {
	// The path is the user's own --profile-path, GLAZE_PATH, current directory or a directory that tmux reports.
	info, err := os.Stat(path) //nolint:gosec // G703

	return err == nil && info.IsDir()
}
