package enums

import "regexp"

type Layout int

// layoutStringPattern matches a tmux window layout coordinate string, e.g.
// "bb62,80x24,0,0" or a nested "e5be,80x24,0,0{40x24,0,0,1,39x24,41,0,2}".
// It is a structural guard only: the leading four-hex checksum is computed by
// tmux over the remainder of the string, so we cannot verify it is correct
// here - a well-formed but stale checksum is rejected by tmux itself at `up`.
var layoutStringPattern = regexp.MustCompile(`^[0-9a-f]{4},[0-9]+x[0-9]+,[0-9]+,[0-9]+[0-9x,{}\[\]]*$`)

// IsLayoutString reports whether s is a structurally valid tmux layout
// coordinate string (as opposed to one of the named layout presets).
func IsLayoutString(s string) bool {
	return layoutStringPattern.MatchString(s)
}

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

var LayoutList = []string{
	LayoutEvenHorizontalString,
	LayoutEvenVerticalString,
	LayoutMainHorizontalString,
	LayoutMainVerticalString,
	LayoutTiledString,
}

// String is responsible for returning the string representation of a Layout.
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

// LayoutFromString is responsible for converting a string to a Layout enum.
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
