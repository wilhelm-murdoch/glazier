package tmux

import "strings"

// OptionTables records which tmux option table holds each option name.
type OptionTables struct {
	session map[string]bool
	window  map[string]bool
}

// IsSessionOnly reports whether tmux keeps the option in the session table only.
func (t OptionTables) IsSessionOnly(name string) bool {
	return t.session[name] && !t.window[name]
}

// IsWindowOnly reports whether tmux keeps the option in the window table only, which also holds the pane options.
func (t OptionTables) IsWindowOnly(name string) bool {
	return t.window[name] && !t.session[name]
}

// OptionTables reads the names of the global session and window options from tmux.
func (c Client) OptionTables() (OptionTables, error) {
	session, err := c.optionNames("-g")
	if err != nil {
		return OptionTables{}, err
	}

	window, err := c.optionNames("-gw")
	if err != nil {
		return OptionTables{}, err
	}

	return OptionTables{session: session, window: window}, nil
}

// optionNames returns the option names that show-options lists for the given flags.
func (c Client) optionNames(flags string) (map[string]bool, error) {
	output, err := c.output("show-options", flags)
	if err != nil {
		return nil, err
	}

	names := make(map[string]bool)
	for line := range strings.SplitSeq(output, "\n") {
		name, _, _ := strings.Cut(line, " ")

		// An array option such as status-format[0] has one line for each element.
		name, _, _ = strings.Cut(name, "[")
		if name != "" {
			names[name] = true
		}
	}

	return names, nil
}
