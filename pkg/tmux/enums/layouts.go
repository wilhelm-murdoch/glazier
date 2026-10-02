package enums

import "regexp"

type Layout int

const (
	LayoutEvenHorizontal Layout = iota + 1
	LayoutEvenVertical
	LayoutMainHorizontal
	LayoutMainVertical
	LayoutTiled
	LayoutUnknown
)

const (
	LayoutEvenHorizontalString = "even-horizontal"
	LayoutEvenVerticalString   = "even-vertical"
	LayoutMainHorizontalString = "main-horizontal"
	LayoutMainVerticalString   = "main-vertical"
	LayoutTiledString          = "tiled"
	LayoutUnknownString        = "unknown"
)

var (
	LayoutList = []string{
		LayoutEvenHorizontalString,
		LayoutEvenVerticalString,
		LayoutMainHorizontalString,
		LayoutMainVerticalString,
		LayoutTiledString,
	}

	// layoutStringPattern matches the structure of a tmux layout string, for example "bb62,80x24,0,0".
	// It cannot check the checksum, so tmux rejects a stale one at `up`.
	layoutStringPattern = regexp.MustCompile(`^[0-9a-f]{4},[0-9]+x[0-9]+,[0-9]+,[0-9]+[0-9x,{}\[\]]*$`)

	// layoutCellPattern matches the WxH,X,Y header of each cell in a raw layout string.
	layoutCellPattern = regexp.MustCompile(`[0-9]+x[0-9]+,[0-9]+,[0-9]+`)
)

// String returns the name of the layout preset.
func (l Layout) String() string {
	switch l {
	case LayoutEvenHorizontal:
		return LayoutEvenHorizontalString
	case LayoutEvenVertical:
		return LayoutEvenVerticalString
	case LayoutMainHorizontal:
		return LayoutMainHorizontalString
	case LayoutMainVertical:
		return LayoutMainVerticalString
	case LayoutTiled:
		return LayoutTiledString
	}

	return LayoutUnknownString
}

// LayoutFromString returns the preset with the name s, or LayoutUnknown for a raw layout string or an unknown name.
func LayoutFromString(s string) Layout {
	switch s {
	case LayoutEvenHorizontalString:
		return LayoutEvenHorizontal
	case LayoutEvenVerticalString:
		return LayoutEvenVertical
	case LayoutMainHorizontalString:
		return LayoutMainHorizontal
	case LayoutMainVerticalString:
		return LayoutMainVertical
	case LayoutTiledString:
		return LayoutTiled
	}

	return LayoutUnknown
}

// LayoutCellCount returns how many panes a raw tmux layout string describes.
// Each cell has a WxH,X,Y header; a cell that holds other cells continues with { or [, so it does not count.
func LayoutCellCount(s string) int {
	count := 0
	for _, match := range layoutCellPattern.FindAllStringIndex(s, -1) {
		if end := match[1]; end == len(s) || (s[end] != '{' && s[end] != '[') {
			count++
		}
	}

	return count
}

// IsLayoutString reports whether s is a structurally valid tmux layout
// coordinate string (as opposed to one of the named layout presets).
func IsLayoutString(s string) bool {
	return layoutStringPattern.MatchString(s)
}
