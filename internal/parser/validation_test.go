package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wilhelm-murdoch/glazier/internal/spec"
)

// windowProfile returns a profile with one window whose body is window, in a session whose extra lines are session.
func windowProfile(session, window string) string {
	return "session {\n  name = \"demo\"\n" + session + "  window {\n" + window + "  }\n}\n"
}

func TestDecodeRejectsValuesThatUpCannotApply(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile string
		detail  string
		line    int
	}{
		{"a null command", windowProfile("", "    pane {\n      commands = [\"ls\", null]\n    }\n"), "index 1", 5},
		{"a null session command", windowProfile("  commands = [null]\n", "    pane {}\n"), "index 0", 3},
		{"a null option", windowProfile("", "    options = { \"automatic-rename\" = null }\n    pane {}\n"), `key "automatic-rename"`, 4},
		{"a null env", windowProfile("  envs = { A = null }\n", "    pane {}\n"), `key "A"`, 3},
		{"a null hook command", windowProfile("", "    hooks = { \"window-renamed\" = null }\n    pane {}\n"), `key "window-renamed"`, 4},
		{"an unknown hook", windowProfile("  hooks = { \"session-create\" = \"echo\" }\n", "    pane {}\n"), `"session-create"`, 3},
		{"the direction unknown", windowProfile("", "    pane {\n      adjust {\n        direction = \"unknown\"\n        amount    = \"5\"\n      }\n    }\n"), "not supported", 6},
		{"a null direction", windowProfile("", "    pane {\n      adjust {\n        direction = null\n        amount    = \"5\"\n      }\n    }\n"), "must have a value", 6},
		{"a null amount", windowProfile("", "    pane {\n      adjust {\n        direction = \"up\"\n        amount    = null\n      }\n    }\n"), "must have a value", 7},
		{"an empty session name", "session {\n  name = \"\"\n  window {\n    pane {}\n  }\n}\n", "must not be empty", 2},
		{"a raw layout with more panes than the window", windowProfile("", "    layout = \"e5be,80x24,0,0{40x24,0,0,1,39x24,41,0,2}\"\n    pane {}\n"), "describes 2 panes, but the window declares 1", 3}, // the window block, which holds the layout and the panes
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, diags := New(writeGlaze(t, tc.profile))
			require.False(t, diags.HasErrors(), diags.Error())

			// Before, several of these profiles panicked in the decoder.
			_, diags = p.Decode(spec.Session(""), BuildEvalContext(nil))
			require.True(t, diags.HasErrors())
			assert.Contains(t, diags.Error(), tc.detail)
			require.NotNil(t, diags[0].Subject)
			assert.Equal(t, tc.line, diags[0].Subject.Start.Line)
		})
	}
}

func TestDecodeAcceptsValidHooksAndLayouts(t *testing.T) {
	profile := windowProfile(
		"  hooks = { \"session-created\" = \"echo\", \"after-new-window[1]\" = \"echo\" }\n",
		"    layout = \"e5be,80x24,0,0{40x24,0,0,1,39x24,41,0,2}\"\n    pane {}\n    pane {}\n",
	)

	p, diags := New(writeGlaze(t, profile))
	require.False(t, diags.HasErrors())

	_, diags = p.Decode(spec.Session(""), BuildEvalContext(nil))
	assert.False(t, diags.HasErrors(), diags.Error())
}
