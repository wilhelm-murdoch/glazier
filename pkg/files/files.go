package files

import (
	"errors"
	"fmt"
	"io/fs"
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

// FileExists is a utility function that simply checks if the given path is not only a file, but that it exists and is readable.
func FileExists(path string) bool {
	// The path is the user's own --profile-path/GLAZE_PATH input to a local
	// CLI; there is no privilege boundary to traverse.
	fileInfo, err := os.Stat(path) //nolint:gosec // G703
	if err != nil || errors.Is(err, fs.ErrNotExist) || fileInfo.IsDir() {
		return false
	}

	return true
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

// ResolveProfilePath returns the profile from --profile-path, the current directory or GLAZE_PATH, in that order.
func ResolveProfilePath(profilePath string) (string, error) {
	if profilePath != "" {
		expanded, err := ExpandPath(profilePath)
		if err != nil {
			return profilePath, fmt.Errorf("%w: %w", ErrProfileNotFound, err)
		}

		if isDir(expanded) {
			return profilePath, fmt.Errorf("%w: `%s` is a directory; --profile-path must name a profile file", ErrProfileNotFound, profilePath)
		}

		if !FileExists(expanded) {
			return profilePath, fmt.Errorf("%w: `%s` does not exist", ErrProfileNotFound, profilePath)
		}

		return expanded, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return profilePath, fmt.Errorf("could not read current working directory: %w", err)
	}

	local := filepath.Join(cwd, ".glaze")
	if FileExists(local) {
		return local, nil
	}

	if isDir(local) {
		return local, fmt.Errorf("%w: `%s` is a directory, not a profile", ErrProfileNotFound, local)
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

		if isDir(profile) {
			return profile, fmt.Errorf("%w: `%s` is a directory, not a profile", ErrProfileNotFound, profile)
		}
	}

	return local, fmt.Errorf(
		"%w:\n - tried using --profile-path\n - searching the current directory\n - looking up GLAZE_PATH environment variable",
		ErrProfileNotFound,
	)
}

// isDir reports whether path, after symlinks, is a directory.
func isDir(path string) bool {
	// The path is the user's own --profile-path, GLAZE_PATH or current directory.
	info, err := os.Stat(path) //nolint:gosec // G703

	return err == nil && info.IsDir()
}
