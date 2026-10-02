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

// AdjustmentList is every direction that a profile can use. It leaves out "unknown", which only marks a value that is not in the list.
var AdjustmentList = []string{
	AdjustmentUpString,
	AdjustmentDownString,
	AdjustmentLeftString,
	AdjustmentRightString,
}

// String returns the name of the direction.
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

// AdjustmentFromString returns the direction with the name s.
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

// ResizeFlag returns the resize-pane flag for the direction (-U, -D, -L or -R), and false for an unknown direction.
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
