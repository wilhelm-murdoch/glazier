package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
)

const (
	// RegistryEnv names the environment variable that replaces the XDG rule for the profile registry.
	RegistryEnv = "GLAZE_REGISTRY"

	// ProfileSuffix is the suffix of a profile in the registry; a name never includes it.
	ProfileSuffix = ".glaze"

	// maxProfileNameLength keeps `<name>.glaze` under the name limit of every file system.
	maxProfileNameLength = 128
)

var (
	// ErrInvalidProfileName means that a word is not a profile name, for example because it contains a slash.
	ErrInvalidProfileName = errors.New("not a profile name")

	// ErrRegistryUnusable means that the profile registry cannot be read: its variable is wrong, it is a file or it cannot be opened.
	ErrRegistryUnusable = errors.New("the profile registry cannot be used")

	// profileNamePattern is the closed grammar of a profile name, after the checks that give a more exact message.
	profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
)

// Profile is one entry of the profile registry.
type Profile struct {
	Name string
	Path string
}

// SkippedEntry is a `.glaze` file in the registry that `up` cannot use, with the reason.
type SkippedEntry struct {
	File   string
	Reason string
}

// RegistryDir returns the profile registry: $GLAZE_REGISTRY, else $XDG_CONFIG_HOME/glaze, else ~/.config/glaze.
// It does not use os.UserConfigDir, which ignores XDG_CONFIG_HOME on macOS and would move the registry there.
func RegistryDir() (string, error) {
	if dir := os.Getenv(RegistryEnv); dir != "" {
		expanded, err := ExpandPath(dir)
		if err != nil {
			return "", fmt.Errorf("%w: %s is not valid: %w", ErrRegistryUnusable, RegistryEnv, err)
		}

		if !filepath.IsAbs(expanded) {
			return "", fmt.Errorf("%w: %s must be an absolute path, but it is `%s`", ErrRegistryUnusable, RegistryEnv, dir)
		}

		return filepath.Clean(expanded), nil
	}

	// The XDG base directory specification says to ignore a relative XDG_CONFIG_HOME.
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "glaze"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: %w; set HOME, XDG_CONFIG_HOME or %s", ErrRegistryUnusable, err, RegistryEnv)
	}

	return filepath.Join(home, ".config", "glaze"), nil
}

// IgnoredConfigHome returns a relative XDG_CONFIG_HOME, which the registry rule ignores, and false when the registry does not depend on it.
func IgnoredConfigHome() (string, bool) {
	if os.Getenv(RegistryEnv) != "" {
		return "", false
	}

	dir := os.Getenv("XDG_CONFIG_HOME")

	return dir, dir != "" && !filepath.IsAbs(dir)
}

// registryFix names the variable to change when the registry path is not a directory, which depends on where the path came from.
func registryFix() string {
	switch {
	case os.Getenv(RegistryEnv) != "":
		return "set " + RegistryEnv + " to a directory"
	case filepath.IsAbs(os.Getenv("XDG_CONFIG_HOME")):
		return "set XDG_CONFIG_HOME to a directory, or set " + RegistryEnv
	default:
		return "move it away, or set XDG_CONFIG_HOME or " + RegistryEnv + " to a directory"
	}
}

// CheckRegistryDir reports whether dir can be the profile registry: a missing one is a plain fs.ErrNotExist, every other problem wraps ErrRegistryUnusable.
// A parent that is a file gives ENOTDIR, which is not fs.ErrNotExist, so it has its own message.
func CheckRegistryDir(dir string) error {
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// Stat follows a symlink, so a dangling link looks missing; `mkdir -p` would then fail on the link.
		if target, linkErr := os.Readlink(dir); linkErr == nil {
			return fmt.Errorf("%w: `%s` is a symlink to `%s`, which does not exist; %s", ErrRegistryUnusable, dir, target, registryFix())
		}

		return err
	case errors.Is(err, syscall.ENOTDIR):
		return fmt.Errorf("%w: `%s` cannot be the profile registry, because a part of its path is a file; %s", ErrRegistryUnusable, dir, registryFix())
	case err != nil:
		return fmt.Errorf("%w: could not read `%s`: %w", ErrRegistryUnusable, dir, err)
	case !info.IsDir():
		return fmt.Errorf("%w: `%s` is a file, not the profile registry directory; %s", ErrRegistryUnusable, dir, registryFix())
	}

	return nil
}

// ValidateProfileName rejects a word that is not a profile name, with the fix in the message.
// The checks with the more exact message come first, so `work.glaze` names the suffix and not the grammar.
func ValidateProfileName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("a profile name cannot be empty (%w)", ErrInvalidProfileName)
	case strings.Contains(name, "/"):
		return fmt.Errorf("`%s` is %w: a name cannot contain `/`; for a file, use --profile-path", name, ErrInvalidProfileName)
	case strings.HasPrefix(name, "~"):
		return fmt.Errorf("`%s` is %w: a name cannot start with `~`; for a file, use --profile-path", name, ErrInvalidProfileName)
	case strings.HasPrefix(name, "."):
		return fmt.Errorf("`%s` is %w: a name cannot start with `.`", name, ErrInvalidProfileName)
	case strings.HasPrefix(name, "-"):
		return fmt.Errorf("`%s` is %w: a name cannot start with `-`", name, ErrInvalidProfileName)
	case strings.HasSuffix(name, ProfileSuffix):
		stem := strings.TrimSuffix(name, ProfileSuffix)
		return fmt.Errorf("`%s` is %w: leave out the `%s` suffix (the name is `%s`); for a file, use --profile-path %s",
			name, ErrInvalidProfileName, ProfileSuffix, stem, name)
	case len(name) > maxProfileNameLength:
		return fmt.Errorf("`%s` is %w: a name can have at most %d characters", name, ErrInvalidProfileName, maxProfileNameLength)
	case !profileNamePattern.MatchString(name):
		return fmt.Errorf("`%s` is %w: a name can contain only ASCII letters, digits, `.`, `_` and `-`", name, ErrInvalidProfileName)
	}

	return nil
}

// RegistryProfilePath returns the path of name in the profile registry without reading the file system.
func RegistryProfilePath(name string) (string, error) {
	if err := ValidateProfileName(name); err != nil {
		return "", err
	}

	dir, err := RegistryDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, name+ProfileSuffix), nil
}

// ListProfiles returns the profiles of the registry dir, sorted by name, and the `.glaze` entries that `up` cannot use.
// A file without the suffix, a hidden file and a directory are left out without a word.
func ListProfiles(dir string) ([]Profile, []SkippedEntry, error) {
	if err := CheckRegistryDir(dir); err != nil {
		return nil, nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: could not read `%s`: %w", ErrRegistryUnusable, dir, err)
	}

	var profiles []Profile
	var skipped []SkippedEntry
	for _, entry := range entries {
		file := entry.Name()
		if !strings.HasSuffix(file, ProfileSuffix) || strings.HasPrefix(file, ".") || entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, file)
		name := strings.TrimSuffix(file, ProfileSuffix)
		if err := ValidateProfileName(name); err != nil {
			skipped = append(skipped, SkippedEntry{File: path, Reason: err.Error()})
			continue
		}

		// Stat follows a symlink, so a link to a directory is left out like a directory, and a link to nothing is reported.
		info, err := os.Stat(path) // #nosec G304 -- an entry of the user's own registry
		switch {
		case errors.Is(err, fs.ErrNotExist):
			target, _ := os.Readlink(path)
			skipped = append(skipped, SkippedEntry{File: path, Reason: fmt.Sprintf("a symlink to `%s`, which does not exist", target)})
			continue
		case err != nil:
			skipped = append(skipped, SkippedEntry{File: path, Reason: err.Error()})
			continue
		case info.IsDir():
			continue
		case !info.Mode().IsRegular():
			skipped = append(skipped, SkippedEntry{File: path, Reason: "not a regular file"})
			continue
		}

		profiles = append(profiles, Profile{Name: name, Path: path})
	}

	// ReadDir sorts by file name, where `a-b.glaze` comes before `a.glaze`; the names sort the other way.
	slices.SortFunc(profiles, func(a, b Profile) int { return strings.Compare(a.Name, b.Name) })

	return profiles, skipped, nil
}

// resolveRegistryProfile returns `<registry>/<name>.glaze`, or an error that names the cause and the fix.
// A name reads only the registry; the current directory and GLAZE_PATH appear only in a hint.
func resolveRegistryProfile(name string) (string, error) {
	if err := ValidateProfileName(name); err != nil {
		return "", err
	}

	// A registry that cannot be used is both sentinels, so a profile command exits 3 for a profile it cannot find.
	dir, err := RegistryDir()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrProfileNotFound, err)
	}

	path := filepath.Join(dir, name+ProfileSuffix)

	switch err := CheckRegistryDir(dir); {
	case errors.Is(err, fs.ErrNotExist):
		return path, fmt.Errorf("%w: no profile named `%s`; the profile registry `%s` does not exist\n - `glaze save %s` in a tmux session creates the registry and writes the profile\n - or create it with `mkdir -p %s` and put `%s%s` in it\n - --profile-path reads a profile from any path%s",
			ErrProfileNotFound, name, dir, name, dir, name, ProfileSuffix, missingHints(name, dir))
	case err != nil:
		return path, fmt.Errorf("%w: %w", ErrProfileNotFound, err)
	}

	linkInfo, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return path, missingProfile(name, dir, path)
	}

	if err != nil {
		return path, fmt.Errorf("%w: %w: could not read `%s`: %w", ErrProfileNotFound, ErrRegistryUnusable, path, err)
	}

	if linkInfo.Mode()&os.ModeSymlink != 0 {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			target, _ := os.Readlink(path)
			return path, fmt.Errorf("%w: `%s` is a symlink to `%s`, which does not exist", ErrProfileNotFound, path, target)
		}
	}

	if isDir(path) {
		return path, fmt.Errorf("%w: `%s` is a directory, not a profile", ErrProfileNotFound, path)
	}

	return path, nil
}

// missingProfile is the error for a well-formed name without a file, with a hint when a file of that name exists elsewhere.
func missingProfile(name, dir, path string) error {
	return fmt.Errorf("%w: no profile named `%s` in the profile registry; `%s` does not exist\n - `glaze ls --profiles` lists the profiles in `%s`\n - `glaze save %s` in a tmux session writes the profile\n - --profile-path reads a profile from any path%s",
		ErrProfileNotFound, name, path, dir, name, missingHints(name, dir))
}

// missingHints says when a file of the name exists in the current directory or in GLAZE_PATH, which a name never reads.
// Both hints are read-only and best effort: without a current directory or a GLAZE_PATH there is nothing to say.
func missingHints(name, dir string) string {
	file := name + ProfileSuffix
	hints := ""

	if cwd, err := os.Getwd(); err == nil && FileExists(filepath.Join(cwd, file)) {
		hints += fmt.Sprintf("\n - `%s` is in the current directory; a name never reads the current directory, so use --profile-path %s", file, file)
	}

	if glazePath := os.Getenv("GLAZE_PATH"); glazePath != "" {
		if expanded, err := ExpandPath(glazePath); err == nil && FileExists(filepath.Join(expanded, file)) {
			hints += fmt.Sprintf("\n - `%s` exists in GLAZE_PATH; GLAZE_PATH is not the profile registry, so move the file to `%s` or set %s",
				filepath.Join(expanded, file), dir, RegistryEnv)
		}
	}

	return hints
}
