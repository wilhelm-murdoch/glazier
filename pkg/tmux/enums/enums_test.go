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

func TestIsLayoutString(t *testing.T) {
	valid := []string{
		"bb62,80x24,0,0",
		"e5be,80x24,0,0{40x24,0,0,1,39x24,41,0,2}",
		"a1b2,200x50,0,0[200x25,0,0,1,200x24,0,26,2]",
	}

	for _, s := range valid {
		if !IsLayoutString(s) {
			t.Errorf("expected %q to be a valid layout string", s)
		}
	}

	invalid := []string{
		"",
		"tiled",
		"main-vertical",
		"zzzz,80x24,0,0", // checksum not hex
		"bb62,80x24",     // missing coordinates
		"bb62;80x24;0;0", // wrong delimiter
		"rm -rf /",       // arbitrary string
	}

	for _, s := range invalid {
		if IsLayoutString(s) {
			t.Errorf("expected %q to be rejected as a layout string", s)
		}
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

func TestIsHook(t *testing.T) {
	for _, name := range []string{"session-created", "pane-died", "client-light-theme", "after-new-window", "after-split-window", "session-created[1]", "pane-exited[12]"} {
		if !IsHook(name) {
			t.Errorf("IsHook(%q) is false, want true", name)
		}
	}

	for _, name := range []string{"", "unknown", "session-create", "after-nothing", "session-created[x]", "[1]", "Session-Created"} {
		if IsHook(name) {
			t.Errorf("IsHook(%q) is true, want false", name)
		}
	}
}

func TestAdjustmentListLeavesOutUnknown(t *testing.T) {
	for _, s := range AdjustmentList {
		if s == AdjustmentUnknownString {
			t.Errorf("AdjustmentList contains %q, which a profile must not use", s)
		}
	}
}

func TestLayoutCellCount(t *testing.T) {
	for layout, want := range map[string]int{
		// Real layouts from tmux 3.7c.
		"aca3,200x50,0,0,6":                           1,
		"e55d,200x50,0,0[200x25,0,0,4,200x24,0,26,5]": 2,
		"8247,200x50,0,0{100x50,0,0,0,99x50,101,0[99x25,101,0,1,99x24,101,26{49x24,101,26,2,49x24,151,26,3}]}": 4,
		// tmux also accepts a cell without a pane id.
		"bb62,80x24,0,0":                       1,
		"e5be,80x24,0,0{40x24,0,0,39x24,41,0}": 2,
	} {
		if got := LayoutCellCount(layout); got != want {
			t.Errorf("LayoutCellCount(%q) = %d, want %d", layout, got, want)
		}
	}
}

// jsonTwoPanes and jsonThreePanes are layouts that tmux 3.9 (next-3.9) printed.
const (
	jsonTwoPanes   = `{"V":2,"L":{"t":"h","w":80,"h":24,"x":0,"y":0,"c":[{"t":"p","w":40,"h":24,"x":0,"y":0,"l":0,"i":0,"I":"%0"},{"t":"p","w":39,"h":24,"x":41,"y":0,"a":true,"i":1,"I":"%1"}]}}`
	jsonThreePanes = `{"V":2,"L":{"t":"h","w":80,"h":24,"x":0,"y":0,"c":[{"t":"p","w":40,"h":24,"x":0,"y":0,"l":1,"i":0,"I":"%0"},{"t":"v","w":39,"h":24,"x":41,"y":0,"c":[{"t":"p","w":39,"h":12,"x":41,"y":0,"l":0,"i":1,"I":"%1"},{"t":"p","w":39,"h":11,"x":41,"y":13,"a":true,"i":2,"I":"%2"}]}]}}`
)

func TestJSONLayout(t *testing.T) {
	for layout, panes := range map[string]int{jsonTwoPanes: 2, jsonThreePanes: 3} {
		if !IsLayoutString(layout) {
			t.Errorf("IsLayoutString(%q) = false", layout)
		}

		if got := LayoutCellCount(layout); got != panes {
			t.Errorf("LayoutCellCount(%q) = %d, want %d", layout, got, panes)
		}
	}

	for name, layout := range map[string]string{
		"not JSON":              `{"V":2,`,
		"no root":               `{"V":2}`,
		"a pane without size":   `{"V":2,"L":{"t":"p","w":0,"h":24}}`,
		"a split without cells": `{"V":2,"L":{"t":"h","w":80,"h":24}}`,
		"a pane with cells":     `{"V":2,"L":{"t":"p","w":80,"h":24,"c":[{"t":"p","w":80,"h":24}]}}`,
		"an unknown cell type":  `{"V":2,"L":{"t":"x","w":80,"h":24}}`,
		"a bad cell inside":     `{"V":2,"L":{"t":"h","w":80,"h":24,"c":[{"t":"p","w":40,"h":-1}]}}`,
		"a JSON array":          `[{"V":2}]`,
	} {
		if IsLayoutString(layout) {
			t.Errorf("%s: IsLayoutString(%q) = true", name, layout)
		}
	}
}
