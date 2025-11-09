package tmux

import (
	"os"
	"os/exec"
)

// IsInstalled returns true if tmux is installed on the system and also returns
// the path to the associated binary.
func IsInstalled() (bool, string) {
	path, err := exec.LookPath("tmux")
	return err == nil, path
}

// IsInsideTmux checks if we are inside a tmux session. We assume we are in
// a tmux session when the `$TMUX` environment variable is set.
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}
