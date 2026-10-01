// Package term decides whether glaze may write colour to a stream.
package term

import (
	"os"

	"github.com/mattn/go-isatty"
)

// ColorEnabled reports whether glaze may write colour to f. NO_COLOR (when not empty) and TERM=dumb turn colour off.
func ColorEnabled(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}

	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}
