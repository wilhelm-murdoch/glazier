package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// ExpandPath is a utility function that determines whether the given path is a shortcut to a user's home directory. If it is it returns the associated absolute path.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		return strings.Replace(path, "~", userHome, 1)
	}

	return path
}

// ErrProfileNotFound means that glaze cannot find the profile to read.
var ErrProfileNotFound = errors.New("glaze profile not found")

// ResolveProfilePath returns the profile from --profile-path, the current directory or GLAZE_PATH, in that order.
func ResolveProfilePath(profilePath string) (string, error) {
	if profilePath != "" {
		if exists := FileExists(profilePath); !exists {
			return profilePath, fmt.Errorf("%w: `%s` does not exist", ErrProfileNotFound, profilePath)
		}

		return ExpandPath(profilePath), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return profilePath, fmt.Errorf("could not read current working directory: %w", err)
	}

	profilePath = filepath.Join(cwd, ".glaze")
	if !FileExists(profilePath) && os.Getenv("GLAZE_PATH") != "" {
		profilePath = filepath.Join(os.Getenv("GLAZE_PATH"), ".glaze")
	}

	if !FileExists(profilePath) {
		return profilePath, fmt.Errorf(
			"%w:\n - tried using --profile-path\n - searching the current directory\n - looking up GLAZE_PATH environment variable",
			ErrProfileNotFound,
		)
	}

	return profilePath, nil
}
