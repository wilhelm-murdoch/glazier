package tmux

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// MinVersion is the oldest tmux that glaze supports, which is the oldest target of the end-to-end matrix.
var MinVersion = Version{Major: 3, Minor: 2, Patch: "a"}

// versionPattern matches the number in "tmux 3.2a", "tmux 3.7c" and "tmux next-3.9", the forms that `tmux -V` prints.
var versionPattern = regexp.MustCompile(`^tmux (?:next-)?(\d+)\.(\d+)([a-z]?)`)

// Version is a tmux release number such as 3.2a: a major, a minor and an optional patch letter.
type Version struct {
	Major int
	Minor int
	Patch string
}

// ParseVersion reads the version that `tmux -V` printed and reports false for a string that glaze does not know.
func ParseVersion(output string) (Version, bool) {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if match == nil {
		return Version{}, false
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])

	return Version{Major: major, Minor: minor, Patch: match[3]}, true
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d%s", v.Major, v.Minor, v.Patch)
}

// Compare returns -1 when v is older than other, 0 when they are equal and 1 when v is newer. A patch letter is newer than none.
func (v Version) Compare(other Version) int {
	if c := cmp.Compare(v.Major, other.Major); c != 0 {
		return c
	}

	if c := cmp.Compare(v.Minor, other.Minor); c != 0 {
		return c
	}

	return strings.Compare(v.Patch, other.Patch)
}

// Version runs `tmux -V` and returns the version of the tmux binary, which needs no server.
func (c Client) Version() (Version, error) {
	output, err := c.output("-V")
	if err != nil {
		return Version{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}

	version, ok := ParseVersion(output)
	if !ok {
		return Version{}, fmt.Errorf("%w: `%s`", ErrUnknownVersion, strings.TrimSpace(output))
	}

	return version, nil
}

// CheckVersion returns ErrOldTmux for a tmux older than MinVersion.
// A version that glaze cannot read only warns, because a fork can print its own string.
func (c Client) CheckVersion() error {
	version, err := c.Version()
	if errors.Is(err, ErrUnknownVersion) {
		c.logger.Warn("could not read the tmux version; glaze needs tmux "+MinVersion.String()+" or later", "error", err)
		return nil
	}

	if err != nil {
		return err
	}

	if version.Compare(MinVersion) < 0 {
		return fmt.Errorf("%w: found tmux %s, glaze needs tmux %s or later", ErrOldTmux, version, MinVersion)
	}

	return nil
}
