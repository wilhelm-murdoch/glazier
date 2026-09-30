package tmux

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	formatActiveSessions       = "#{session_id};#{q:session_name};#{q:session_path}"
	formatActiveWindows        = "#{window_id};#{window_index};#{q:window_name};#{window_layout};#{window_active}"
	formatActivePanes          = "#{pane_id};#{pane_index};#{q:pane_title};#{pane_active};#{q:pane_current_path}"
	tmuxLinePartDelimiter byte = ';'
)

var (
	// ErrTrailingEscape is returned when a line ends in a lone backslash,
	// i.e. an escape with nothing to escape. tmux never produces this, so
	// it signals a truncated or corrupted line rather than real data.
	ErrTrailingEscape = errors.New("tmux: line ends in an unpaired escape character")

	// ErrUnexpectedPartCount is returned when the number of derived parts
	// of a tmux line do not equal the expected amount.
	ErrUnexpectedPartCount = errors.New("tmux: unexpected number of parts in line")

	// ErrInvalidDerivedId is returned when a suitable id cannot be
	// derived from the given part.
	ErrInvalidDerivedId = errors.New("tmux: line has an invalid id")

	// ErrIdPrefixNotFound is returned when a specified prefix cannot be
	// found within the get part.
	ErrIdPrefixNotFound = errors.New("tmux: id prefix not found")
)

func splitTmuxLine(line string, delimiter byte) ([]string, error) {
	var (
		parts     []string
		part      strings.Builder
		isEscaped bool
	)

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case isEscaped:
			part.WriteByte(c)
			isEscaped = false
		case c == '\\':
			isEscaped = true
		case c == delimiter:
			parts = append(parts, part.String())
			part.Reset()
		default:
			part.WriteByte(c)
		}
	}

	if isEscaped {
		return nil, ErrTrailingEscape
	}

	// The last field has no trailing delimiter, so store it here.
	return append(parts, part.String()), nil
}

// undoDollarEscape removes the backslash in front of each $ in a line that
// tmux printed with #{q:...} fields. This is only safe for q: fields. A
// plain #{name} field does not escape $, so a real backslash in front of
// $ would be removed.
func undoDollarEscape(line string) string {
	return strings.ReplaceAll(line, `\$`, "$")
}

func getPartsFromTmuxLine(line, prefix string, expectedLength int) ([]string, int, error) {
	parts, err := splitTmuxLine(undoDollarEscape(line), tmuxLinePartDelimiter)

	if err != nil {
		return parts, 0, err
	}

	if len(parts) != expectedLength {
		return parts, 0, fmt.Errorf(
			"%w: expected %d, got %d: %q",
			ErrUnexpectedPartCount,
			expectedLength,
			len(parts),
			line,
		)
	}

	rawId, found := strings.CutPrefix(parts[0], prefix)
	if !found {
		return parts, 0, fmt.Errorf("%w: %w: want %q in %q", ErrInvalidDerivedId, ErrIdPrefixNotFound, prefix, parts[0])
	}

	id, err := strconv.Atoi(rawId)
	if err != nil {
		return parts, 0, fmt.Errorf("%w: %w", ErrInvalidDerivedId, err)
	}

	return parts, id, nil
}
