package files

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrNotRegular means that glaze can rewrite only a regular file, not a pipe, a device or a directory.
var ErrNotRegular = errors.New("not a regular file")

// ErrFileExists means that the file exists and the caller did not allow glaze to replace it.
var ErrFileExists = errors.New("file exists")

// IsRegular reports whether path, after symlinks, is a regular file.
func IsRegular(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

// WriteFile replaces path with data through a temporary file and one rename, so a failed write leaves the old file whole.
// It writes through a symlink, keeps the mode of an existing file and refuses a file that the user cannot write.
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	target, err := writeTarget(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(target)
	existing := err == nil

	if existing {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("could not write `%s`: %w", path, ErrNotRegular)
		}

		// An open for write without truncation checks the permission and changes nothing.
		file, err := os.OpenFile(target, os.O_WRONLY, 0) //nolint:gosec // G304: the user's own profile path.
		if err != nil {
			return err
		}

		_ = file.Close()

		perm = info.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	temp, err := createTemp(target, perm)
	if err != nil {
		return err
	}

	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(temp.Name())
		}
	}()

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}

	// A new file keeps the umask from its creation. An existing file gets its exact mode back.
	if existing {
		if err := temp.Chmod(perm); err != nil {
			_ = temp.Close()
			return err
		}
	}

	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}

	if err := temp.Close(); err != nil {
		return err
	}

	if err := os.Rename(temp.Name(), target); err != nil {
		return err
	}

	renamed = true

	return nil
}

// writeTarget returns the file that path names: the target of a symlink, or path itself.
func writeTarget(path string) (string, error) {
	target, err := filepath.EvalSymlinks(path)
	if err == nil {
		return target, nil
	}

	// A path that does not exist is a new file. A symlink whose target does not exist is an error, not a new file.
	if info, lerr := os.Lstat(path); errors.Is(lerr, fs.ErrNotExist) || (lerr == nil && info.Mode()&fs.ModeSymlink == 0) {
		return path, nil
	}

	return "", fmt.Errorf("could not resolve `%s`: %w", path, err)
}

// createTemp creates a new hidden file next to target, so the rename stays on one file system.
func createTemp(target string, perm fs.FileMode) (*os.File, error) {
	dir, base := filepath.Split(target)

	for range 10 {
		name := filepath.Join(dir, fmt.Sprintf(".%s.%s.tmp", base, rand.Text()[:8]))

		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm) //nolint:gosec // G304: next to the user's own profile.
		if !errors.Is(err, fs.ErrExist) {
			return file, err
		}
	}

	return nil, fmt.Errorf("could not create a temporary file next to `%s`", target)
}
