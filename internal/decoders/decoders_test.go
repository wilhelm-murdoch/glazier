package decoders

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux/enums"
)

// nullStr is a null cty string used for omitted attributes.
var nullStr = cty.NullVal(cty.String)

// nullMap is a null cty string map used for omitted map attributes.
var nullMap = cty.NullVal(cty.Map(cty.String))

// adjustType describes the object shape of a single decoded adjust block.
var adjustType = cty.Object(map[string]cty.Type{
	"direction": cty.String,
	"amount":    cty.String,
})

// adjustVal builds a single adjust block value.
func adjustVal(direction, amount string) cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"direction": cty.StringVal(direction),
		"amount":    cty.StringVal(amount),
	})
}

func TestNewBaseDefaults(t *testing.T) {
	spec := cty.ObjectVal(map[string]cty.Value{
		"name":               nullStr,
		"starting_directory": nullStr,
		"envs":               nullMap,
		"hooks":              nullMap,
		"options":            nullMap,
	})

	base := NewBase(spec)

	assert.Equal(t, DefaultGlazeElementName, base.Name)

	pwd, _ := os.Getwd()
	assert.Equal(t, pwd, base.StartingDirectory)
	assert.Nil(t, base.Envs)
	assert.Nil(t, base.Hooks)
	assert.Nil(t, base.Options)
}

func TestNewBasePopulated(t *testing.T) {
	spec := cty.ObjectVal(map[string]cty.Value{
		"name":               cty.StringVal("session-a"),
		"starting_directory": cty.StringVal("/tmp"),
		"envs": cty.MapVal(map[string]cty.Value{
			"EDITOR": cty.StringVal("vim"),
		}),
		"hooks": cty.MapVal(map[string]cty.Value{
			"session-created": cty.StringVal("echo hi"),
		}),
		"options": cty.MapVal(map[string]cty.Value{
			"mouse": cty.StringVal("on"),
		}),
	})

	base := NewBase(spec)

	assert.Equal(t, "session-a", base.Name)
	assert.Equal(t, "/tmp", base.StartingDirectory)
	assert.Equal(t, "vim", base.Envs["EDITOR"])
	assert.Equal(t, "echo hi", base.Hooks["session-created"])
	assert.Equal(t, "on", base.Options["mouse"])
}

// paneSpec builds a cty.Value matching the object shape the pane decoder reads.
func paneSpec(focus cty.Value, size cty.Value, commands cty.Value) cty.Value {
	return paneSpecWithAdjust(focus, size, commands, cty.NullVal(cty.List(adjustType)))
}

// paneSpecWithAdjust is paneSpec with an explicit adjust block list.
func paneSpecWithAdjust(focus cty.Value, size cty.Value, commands cty.Value, adjust cty.Value) cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"name":               nullStr,
		"starting_directory": cty.StringVal("/tmp"),
		"envs":               nullMap,
		"hooks":              nullMap,
		"options":            nullMap,
		"focus":              focus,
		"size":               size,
		"adjust":             adjust,
		"commands":           commands,
	})
}

func sizeVal(x, y string) cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"x": cty.StringVal(x),
		"y": cty.StringVal(y),
	})
}

func TestPaneDecode(t *testing.T) {
	t.Run("decodes focus, size and commands", func(t *testing.T) {
		spec := paneSpec(
			cty.True,
			sizeVal("50%", "100"),
			cty.ListVal([]cty.Value{cty.StringVal("vim"), cty.StringVal("ls")}),
		)

		pane := NewPane(spec)
		diags := pane.Decode()

		assert.False(t, diags.HasErrors())
		assert.True(t, pane.Focus)
		assert.True(t, pane.Size.Valid())
		assert.Equal(t, "50%", pane.Size.X)
		assert.Equal(t, "100", pane.Size.Y)
		assert.Equal(t, []string{"vim", "ls"}, pane.Commands)
	})

	t.Run("handles omitted optional attributes", func(t *testing.T) {
		spec := paneSpec(
			cty.NullVal(cty.Bool),
			cty.NullVal(cty.Object(map[string]cty.Type{"x": cty.String, "y": cty.String})),
			cty.NullVal(cty.List(cty.String)),
		)

		pane := NewPane(spec)
		diags := pane.Decode()

		assert.False(t, diags.HasErrors())
		assert.False(t, pane.Focus)
		assert.False(t, pane.Size.Valid())
		assert.Empty(t, pane.Commands)
	})

	t.Run("handles a size block with only one dimension", func(t *testing.T) {
		spec := paneSpec(
			cty.NullVal(cty.Bool),
			cty.ObjectVal(map[string]cty.Value{
				"x": cty.StringVal("80"),
				"y": cty.NullVal(cty.String),
			}),
			cty.ListValEmpty(cty.String),
		)

		pane := NewPane(spec)
		diags := pane.Decode()

		assert.False(t, diags.HasErrors())
		assert.Equal(t, "80", pane.Size.X)
		assert.Equal(t, "", pane.Size.Y)
		assert.False(t, pane.Size.Valid())
	})
}

func TestPaneDecodeAdjust(t *testing.T) {
	t.Run("decodes adjustments in order", func(t *testing.T) {
		spec := paneSpecWithAdjust(
			cty.NullVal(cty.Bool),
			cty.NullVal(cty.Object(map[string]cty.Type{"x": cty.String, "y": cty.String})),
			cty.ListValEmpty(cty.String),
			cty.ListVal([]cty.Value{
				adjustVal("up", "5"),
				adjustVal("left", "10"),
			}),
		)

		pane := NewPane(spec)
		diags := pane.Decode()

		assert.False(t, diags.HasErrors())
		assert.Len(t, pane.Adjustments, 2)
		assert.Equal(t, enums.AdjustmentUp, pane.Adjustments[0].Direction)
		assert.Equal(t, "5", pane.Adjustments[0].Amount)
		assert.Equal(t, enums.AdjustmentLeft, pane.Adjustments[1].Direction)
		assert.Equal(t, "10", pane.Adjustments[1].Amount)
	})

	t.Run("handles an absent adjust block", func(t *testing.T) {
		spec := paneSpecWithAdjust(
			cty.NullVal(cty.Bool),
			cty.NullVal(cty.Object(map[string]cty.Type{"x": cty.String, "y": cty.String})),
			cty.ListValEmpty(cty.String),
			cty.NullVal(cty.List(adjustType)),
		)

		pane := NewPane(spec)
		diags := pane.Decode()

		assert.False(t, diags.HasErrors())
		assert.Empty(t, pane.Adjustments)
	})
}

func TestSizeValid(t *testing.T) {
	assert.True(t, Size{X: "1", Y: "2"}.Valid())
	assert.False(t, Size{X: "1"}.Valid())
	assert.False(t, Size{Y: "2"}.Valid())
	assert.False(t, Size{}.Valid())
}

// windowSpec builds a cty.Value matching the object shape the window decoder reads.
func windowSpec(layout cty.Value, focus cty.Value, panes cty.Value) cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"name":               nullStr,
		"starting_directory": cty.StringVal("/tmp"),
		"envs":               nullMap,
		"hooks":              nullMap,
		"options":            nullMap,
		"layout":             layout,
		"focus":              focus,
		"panes":              panes,
	})
}

func TestWindowDecode(t *testing.T) {
	onePane := cty.ListVal([]cty.Value{
		paneSpec(cty.True, cty.NullVal(cty.Object(map[string]cty.Type{"x": cty.String, "y": cty.String})), cty.ListVal([]cty.Value{cty.StringVal("htop")})),
	})

	t.Run("decodes layout, focus and panes", func(t *testing.T) {
		spec := windowSpec(cty.StringVal("main-vertical"), cty.True, onePane)

		window := NewWindow(spec)
		diags := window.Decode()

		assert.False(t, diags.HasErrors())
		assert.Equal(t, enums.LayoutMainVertical, window.Layout)
		assert.True(t, window.Focus)
		assert.Equal(t, 1, window.Panes.Length())
		assert.Equal(t, []string{"htop"}, window.Panes.Items()[0].Commands)
	})

	t.Run("defaults layout to tiled when omitted", func(t *testing.T) {
		spec := windowSpec(nullStr, cty.NullVal(cty.Bool), onePane)

		window := NewWindow(spec)
		diags := window.Decode()

		assert.False(t, diags.HasErrors())
		assert.Equal(t, enums.LayoutTiled, window.Layout)
		assert.False(t, window.Focus)
	})
}

func TestSessionDecode(t *testing.T) {
	panes := cty.ListVal([]cty.Value{
		paneSpec(cty.NullVal(cty.Bool), cty.NullVal(cty.Object(map[string]cty.Type{"x": cty.String, "y": cty.String})), cty.ListVal([]cty.Value{cty.StringVal("echo")})),
	})
	windows := cty.ListVal([]cty.Value{
		windowSpec(cty.StringVal("tiled"), cty.NullVal(cty.Bool), panes),
	})

	spec := cty.ObjectVal(map[string]cty.Value{
		"name":               cty.StringVal("demo"),
		"starting_directory": cty.StringVal("/tmp"),
		"envs":               nullMap,
		"hooks":              nullMap,
		"options":            nullMap,
		"commands":           cty.ListVal([]cty.Value{cty.StringVal("echo booting")}),
		"windows":            windows,
	})

	session := NewSession(spec)
	diags := session.Decode()

	assert.False(t, diags.HasErrors())
	assert.Equal(t, "demo", session.Name)
	assert.Equal(t, []string{"echo booting"}, session.Commands)
	assert.Equal(t, 1, session.Windows.Length())
	assert.Equal(t, 1, session.Windows.Items()[0].Panes.Length())
}
