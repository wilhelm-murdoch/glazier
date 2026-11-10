// Package term decides whether glaze may write colour to a stream.
package term

import (
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// ColorEnabled reports whether glaze may write colour to f. NO_COLOR (when not empty) and TERM=dumb turn colour off.
func ColorEnabled(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}

	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// Paint colours s with attrs when enabled is true, and returns s as it is otherwise.
// It forces the colour on, because the caller already decided with ColorEnabled.
func Paint(enabled bool, s string, attrs ...color.Attribute) string {
	if !enabled {
		return s
	}

	c := color.New(attrs...)
	c.EnableColor()

	return c.Sprint(s)
}
