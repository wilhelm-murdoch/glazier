package tmux

import (
	"fmt"
	"strconv"
	"strings"
)

func getPartsFromTmuxLine(
	line, prefix string,
	expectedLength int,
) ([]string, int, error) {
	parts := strings.SplitN(line, ";", expectedLength)

	if len(parts) != expectedLength {
		return parts, 0, fmt.Errorf(
			"expected %d parts for tmux line, but got %d instead: %s",
			expectedLength,
			len(parts),
			line,
		)
	}

	id, err := strconv.Atoi(strings.ReplaceAll(parts[0], prefix, ""))
	if err != nil {
		return parts, 0, err
	}

	return parts, id, nil
}
