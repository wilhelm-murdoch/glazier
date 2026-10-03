package enums

import (
	"encoding/json"
	"regexp"
	"strings"
)

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

// jsonLayout is the layout that tmux 3.9 and later print, for example {"V":2,"L":{"t":"h","w":80,...}}.
// tmux accepts it and the classic string in select-layout, so a raw layout can have either form.
type jsonLayout struct {
	Version int             `json:"V"`
	Root    *jsonLayoutCell `json:"L"`
}

// jsonLayoutCell is one cell of a JSON layout: a pane ("p"), or a split ("h" or "v") that holds other cells.
type jsonLayoutCell struct {
	Type   string           `json:"t"`
	Width  int              `json:"w"`
	Height int              `json:"h"`
	Cells  []jsonLayoutCell `json:"c"`
}

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
// In a JSON layout, each pane is a cell of type "p".
func LayoutCellCount(s string) int {
	if root, ok := parseJSONLayout(s); ok {
		return root.panes()
	}

	count := 0
	for _, match := range layoutCellPattern.FindAllStringIndex(s, -1) {
		if end := match[1]; end == len(s) || (s[end] != '{' && s[end] != '[') {
			count++
		}
	}

	return count
}

// IsLayoutString reports whether s is a structurally valid tmux layout
// coordinate string (as opposed to one of the named layout presets), in the classic or the JSON form.
func IsLayoutString(s string) bool {
	if _, ok := parseJSONLayout(s); ok {
		return true
	}

	return layoutStringPattern.MatchString(s)
}

// parseJSONLayout returns the root cell of a JSON layout, or false when s is not a well-formed one.
// It checks the structure only; tmux checks the geometry at `up`.
func parseJSONLayout(s string) (*jsonLayoutCell, bool) {
	if !strings.HasPrefix(s, "{") {
		return nil, false
	}

	var layout jsonLayout
	if err := json.Unmarshal([]byte(s), &layout); err != nil || layout.Root == nil || !layout.Root.valid() {
		return nil, false
	}

	return layout.Root, true
}

// valid reports whether a cell and every cell in it has a size, and whether only a split holds cells.
func (c jsonLayoutCell) valid() bool {
	if c.Width < 1 || c.Height < 1 {
		return false
	}

	switch c.Type {
	case "p":
		return len(c.Cells) == 0
	case "h", "v":
		if len(c.Cells) == 0 {
			return false
		}

		for _, cell := range c.Cells {
			if !cell.valid() {
				return false
			}
		}

		return true
	default:
		return false
	}
}

// panes returns the number of panes in a cell.
func (c jsonLayoutCell) panes() int {
	if c.Type == "p" {
		return 1
	}

	count := 0
	for _, cell := range c.Cells {
		count += cell.panes()
	}

	return count
}
