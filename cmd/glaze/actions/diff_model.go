package actions

import (
	"strings"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// desired is the session that up would create from a profile, in the shape of the live model.
type desired struct {
	Name              string
	Directory         string
	DirectoryDeclared bool

	// Focus is the index of the window that up would select, or -1 when the profile declares none.
	Focus   int
	Windows []desiredWindow
}

// desiredWindow is a window of the profile; AutomaticDeclared says that the profile asks tmux to name it.
type desiredWindow struct {
	Name              string
	AutomaticDeclared bool
	Focus             int
	Panes             []desiredPane
}

// desiredPane is a pane of the profile; InheritsSessionDirectory says that its directory comes from an undeclared session directory.
type desiredPane struct {
	Name                     string
	Directory                string
	InheritsSessionDirectory bool
}

// directoryFlags records which directories a profile declares, read before ResolveDirectories fills the rest.
type directoryFlags struct {
	session bool
	windows []windowDirectoryFlags
}

// windowDirectoryFlags records whether a window declares a directory, and the same for each of its panes.
type windowDirectoryFlags struct {
	window bool
	panes  []bool
}

// emptyDirectories records which directories the profile leaves out, so the model can say where a value came from.
func emptyDirectories(profile *decoders.Session) directoryFlags {
	flags := directoryFlags{session: profile.StartingDirectory != ""}
	for _, window := range profile.Windows {
		wf := windowDirectoryFlags{window: window.StartingDirectory != ""}
		for _, pane := range window.Panes {
			wf.panes = append(wf.panes, pane.StartingDirectory != "")
		}

		flags.windows = append(flags.windows, wf)
	}

	return flags
}

// desiredSession builds the model that up would create: names as tmux stores them, resolved directories and the focus that up selects last.
func desiredSession(profile *decoders.Session, declared directoryFlags) desired {
	want := desired{
		Name:              tmux.SanitizeSessionName(profile.Name),
		Directory:         profile.StartingDirectory,
		DirectoryDeclared: declared.session,
		Focus:             -1,
	}

	for i, window := range profile.Windows {
		dw := desiredWindow{
			Name:              tmux.SanitizeName(window.Name),
			AutomaticDeclared: automaticRenameOn(window.Options) || (!hasOption(window.Options, "automatic-rename") && automaticRenameOn(profile.Options)),
			Focus:             -1,
		}

		// up selects each focused item in file order, so the last one wins.
		if window.Focus {
			want.Focus = i
		}

		for j, pane := range window.Panes {
			dw.Panes = append(dw.Panes, desiredPane{
				Name:                     tmux.SanitizeName(pane.Name),
				Directory:                pane.StartingDirectory,
				InheritsSessionDirectory: !declared.session && !declared.windows[i].window && !declared.windows[i].panes[j],
			})

			if pane.Focus {
				dw.Focus = j
			}
		}

		want.Windows = append(want.Windows, dw)
	}

	return want
}

// hasOption reports whether the options of a block set the key.
func hasOption(options map[string]string, key string) bool {
	_, ok := options[key]
	return ok
}

// automaticRenameOn reports whether the options of a block turn automatic-rename on.
func automaticRenameOn(options map[string]string) bool {
	switch strings.ToLower(options["automatic-rename"]) {
	case "on", "1", "yes":
		return true
	}

	return false
}
