package enums

type Adjustment int

const (
	AdjustmentUp = iota + 1
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
func AdjustmentFromString(s string) Layout {
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
