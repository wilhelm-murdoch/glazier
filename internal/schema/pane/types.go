package pane

type (
	Value     string         // Value represents a generic string value used in various pane configurations.
	Name      string         // Name represents the name of a pane, environment variable, or hook.
	Directory string         // Directory represents a file system path, typically for a starting directory.
	Envs      map[Name]Value // Envs is a map of environment variable names to their corresponding values.
	Hooks     map[Name]Value // Hooks is a map of hook names to their associated commands or values.
	Options   map[Name]Value // Options is a map of pane-specific options to their values.
	Command   string         // Command represents a command to be executed within a pane.
	Focus     bool           // Focus indicates whether a pane should be focused.
)

type Size struct {
	X, Y string
}

// Valid is a function that returns True if both X and Y values are not blank
func (s Size) Valid() bool {
	return s.X != "" && s.Y != ""
}

// Pane represents the configuration for a single tmux pane.
type Pane struct {
	Name              Name
	StartingDirectory Directory
	Size              Size
	Envs              Envs
	Hooks             Hooks
	Options           Options
	Commands          []Command
	Focus             Focus
}
