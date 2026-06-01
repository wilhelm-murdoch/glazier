package enums

import "testing"

func TestLayoutRoundTrip(t *testing.T) {
	for _, s := range LayoutList {
		if got := LayoutFromString(s).String(); got != s {
			t.Errorf("layout round trip for %q produced %q", s, got)
		}
	}
}

func TestLayoutUnknown(t *testing.T) {
	if LayoutFromString("nonsense") != LayoutUnknown {
		t.Error("expected LayoutUnknown for an unrecognized string")
	}
	if LayoutUnknown.String() != LayoutUnknownString {
		t.Errorf("expected %q, got %q", LayoutUnknownString, LayoutUnknown.String())
	}
}

func TestHookRoundTrip(t *testing.T) {
	for _, s := range HookList {
		if got := HookFromString(s).String(); got != s {
			t.Errorf("hook round trip for %q produced %q", s, got)
		}
	}
}

func TestHookUnknown(t *testing.T) {
	if HookFromString("nonsense") != HookUnknown {
		t.Error("expected HookUnknown for an unrecognized string")
	}
	if HookUnknown.String() != HookUnknownString {
		t.Errorf("expected %q, got %q", HookUnknownString, HookUnknown.String())
	}
}

func TestAdjustmentRoundTrip(t *testing.T) {
	for _, s := range []string{
		AdjustmentUpString,
		AdjustmentDownString,
		AdjustmentLeftString,
		AdjustmentRightString,
	} {
		if got := AdjustmentFromString(s).String(); got != s {
			t.Errorf("adjustment round trip for %q produced %q", s, got)
		}
	}
}

func TestAdjustmentUnknown(t *testing.T) {
	if AdjustmentFromString("nonsense") != AdjustmentUnknown {
		t.Error("expected AdjustmentUnknown for an unrecognized string")
	}
	if AdjustmentUnknown.String() != AdjustmentUnknownString {
		t.Errorf("expected %q, got %q", AdjustmentUnknownString, AdjustmentUnknown.String())
	}
}

func TestAdjustmentResizeFlag(t *testing.T) {
	cases := map[Adjustment]string{
		AdjustmentUp:    "-U",
		AdjustmentDown:  "-D",
		AdjustmentLeft:  "-L",
		AdjustmentRight: "-R",
	}

	for adjustment, want := range cases {
		flag, ok := adjustment.ResizeFlag()
		if !ok {
			t.Errorf("expected a flag for %s", adjustment)
		}
		if flag != want {
			t.Errorf("expected %q for %s, got %q", want, adjustment, flag)
		}
	}

	if _, ok := AdjustmentUnknown.ResizeFlag(); ok {
		t.Error("expected no flag for AdjustmentUnknown")
	}
}
