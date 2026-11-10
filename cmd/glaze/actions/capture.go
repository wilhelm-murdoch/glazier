package actions

import (
	"fmt"

	"github.com/wilhelm-murdoch/glazier/internal/logger"
	"github.com/wilhelm-murdoch/glazier/pkg/files"
	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// savedPane is a live pane as save and diff see it: the name that a profile would hold, and the facts behind it.
type savedPane struct {
	Name              string
	StartingDirectory string
	Focus             bool

	// Title is the pane title as tmux reports it, and HostTitle says that it is the host name, which tmux sets on a new pane.
	Title     string
	HostTitle bool

	// DirectoryMissing says that the directory no longer exists; DirectoryUnknown says that tmux reported none.
	DirectoryMissing bool
	DirectoryUnknown bool
}

// savedWindow is a live window; LiveName and Automatic keep what tmux reports when the window names itself.
type savedWindow struct {
	Name   string
	Layout string
	Focus  bool
	Panes  []savedPane

	LiveName  string
	Automatic bool
}

// savedSession is a live session, the model that save writes and diff compares.
type savedSession struct {
	Name              string
	StartingDirectory string
	DirectoryMissing  bool
	Windows           []savedWindow
}

// captureSession reads the windows and panes of a session into the shared model.
// It drops the two names that tmux chose, the host name as a pane title and a window under automatic-rename, and keeps the facts.
func captureSession(client tmux.Client, session *tmux.Session, log *logger.Logger) (savedSession, error) {
	captured := savedSession{
		Name:              session.Name,
		StartingDirectory: session.StartingDirectory,
		DirectoryMissing:  directoryMissing(session.StartingDirectory),
	}

	// tmux titles a new pane with the host name, so that title says nothing about the pane.
	host, err := session.Host()
	if err != nil {
		return captured, err
	}

	windows, err := client.Windows(session)
	if err != nil {
		return captured, err
	}

	for _, window := range windows {
		log.Debug("capturing window", "name", window.Name)

		// A window that tmux names after its program gets a different name on each run, so the profile leaves it out.
		automatic, err := window.AutomaticName()
		if err != nil {
			return captured, fmt.Errorf("the session changed while glaze read it; run the command again: %w", err)
		}

		sw := savedWindow{
			Name:      window.Name,
			LiveName:  window.Name,
			Automatic: automatic,

			// tmux reports a layout only as a coordinate string, so save keeps the string as it is.
			Layout: window.RawLayout,
			Focus:  window.IsActive,
		}

		if automatic {
			sw.Name = ""
		}

		panes, err := client.Panes(window)
		if err != nil {
			return captured, fmt.Errorf("the session changed while glaze read it; run the command again: %w", err)
		}

		for _, pane := range panes {
			log.Debug("capturing pane", "name", pane.Name)

			sp := savedPane{
				Name:              pane.Name,
				Title:             pane.Name,
				HostTitle:         pane.Name == host,
				StartingDirectory: pane.StartingDirectory,
				DirectoryUnknown:  pane.StartingDirectory == "",
				DirectoryMissing:  directoryMissing(pane.StartingDirectory),
				Focus:             pane.IsActive,
			}

			if sp.HostTitle {
				sp.Name = ""
			}

			sw.Panes = append(sw.Panes, sp)
		}

		captured.Windows = append(captured.Windows, sw)
	}

	return captured, nil
}

// directoryMissing reports whether a directory that tmux reported no longer exists; an empty path is not missing, only unknown.
func directoryMissing(dir string) bool {
	if dir == "" {
		return false
	}

	return !files.IsDir(dir)
}

// pruneMissingDirectories blanks each directory that no longer exists, with a warning, because the profile would fail validation with it.
// A block without a directory uses the directory of its window or session.
func pruneMissingDirectories(captured *savedSession, log *logger.Logger) {
	if captured.DirectoryMissing {
		log.Warn("the directory no longer exists, so the profile leaves it out", "session", captured.Name, "directory", captured.StartingDirectory)
		captured.StartingDirectory = ""
	}

	for w := range captured.Windows {
		for p := range captured.Windows[w].Panes {
			pane := &captured.Windows[w].Panes[p]
			if pane.DirectoryMissing {
				log.Warn("the directory no longer exists, so the profile leaves it out", "name", pane.Title, "directory", pane.StartingDirectory)
				pane.StartingDirectory = ""
			}
		}
	}
}
