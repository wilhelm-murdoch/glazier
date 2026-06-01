package enums

type Adjustment int

const (
	AdjustmentUp Adjustment = iota + 1
	AdjustmentDown
	AdjustmentLeft
	AdjustmentRight
	AdjustmentUnknown
)

const (
	AdjustmentUpString      = "up"
	AdjustmentDownString    = "down"
	AdjustmentLeftString    = "left"
	AdjustmentRightString   = "right"
	AdjustmentUnknownString = "unknown"
)

var AdjustmentList = []string{
	AdjustmentUpString,
	AdjustmentDownString,
	AdjustmentLeftString,
	AdjustmentRightString,
	AdjustmentUnknownString,
}

// String is responsible for returning the string representation of an Adjustment.
func (a Adjustment) String() string {
	switch a {
	case AdjustmentUp:
		return AdjustmentUpString
	case AdjustmentDown:
		return AdjustmentDownString
	case AdjustmentLeft:
		return AdjustmentLeftString
	case AdjustmentRight:
		return AdjustmentRightString
	}

	return AdjustmentUnknownString
}

// LayoutFromString is responsible for converting a string to a Layout enum.
func AdjustmentFromString(s string) Adjustment {
	switch s {
	case AdjustmentUpString:
		return AdjustmentUp
	case AdjustmentDownString:
		return AdjustmentDown
	case AdjustmentLeftString:
		return AdjustmentLeft
	case AdjustmentRightString:
		return AdjustmentRight
	}

	return AdjustmentUnknown
}

// ResizeFlag returns the `tmux resize-pane` flag corresponding to the
// adjustment direction (-U, -D, -L, -R). The second return value is false for
// unknown directions, for which no flag exists.
func (a Adjustment) ResizeFlag() (string, bool) {
	switch a {
	case AdjustmentUp:
		return "-U", true
	case AdjustmentDown:
		return "-D", true
	case AdjustmentLeft:
		return "-L", true
	case AdjustmentRight:
		return "-R", true
	}

	return "", false
}
