// Package tmuxtest provides test doubles for driving a tmux.Client without
// spawning a real tmux process. It is intended for use by packages that build
// on top of tmux.Client (for example the CLI actions) so they can assert the
// sequence of tmux commands a higher-level operation issues.
package tmuxtest

import (
	"strings"
	"sync"
	"testing"

	"github.com/wilhelm-murdoch/glazier/pkg/tmux"
)

// Result is the canned outcome a single faked tmux invocation returns.
type Result struct {
	Output string
	Err    error
	Status int

	// OnExec runs before the command returns, so a test can block a command or react to it.
	OnExec func()
}

// command is a programmable tmux.Commander representing one tmux invocation.
type command struct {
	args     []string
	result   Result
	recorder *Recorder
	index    int
}

func (c *command) String() string { return strings.Join(c.args, " ") }

func (c *command) Exec() error {
	c.run()
	return c.result.Err
}

func (c *command) ExecWithOutput() (string, error) {
	c.run()
	return c.result.Output, c.result.Err
}

func (c *command) ExecWithStatus() int {
	c.run()
	return c.result.Status
}

func (c *command) ExecWithInput(input string) error {
	c.recorder.mu.Lock()
	c.recorder.inputs[c.index] = input
	c.recorder.mu.Unlock()

	c.run()
	return c.result.Err
}

func (c *command) run() {
	if c.result.OnExec != nil {
		c.result.OnExec()
	}
}

// Recorder records every tmux invocation routed through it and returns canned
// results keyed by tmux subcommand (e.g. "ls", "neww", "splitw"). This lets a
// single test exercise an operation that chains several different tmux commands.
type Recorder struct {
	mu       sync.Mutex
	Calls    [][]string
	queues   map[string][]Result
	inputs   map[int]string
	fallback Result
}

// New returns an empty Recorder.
func New() *Recorder {
	return &Recorder{queues: make(map[string][]Result), inputs: make(map[int]string)}
}

// On enqueues a canned result for the given tmux subcommand. Repeated calls for
// the same subcommand are returned in FIFO order.
func (r *Recorder) On(subcommand string, result Result) *Recorder {
	r.queues[subcommand] = append(r.queues[subcommand], result)
	return r
}

// Default sets the result returned when no queued result matches a subcommand.
func (r *Recorder) Default(result Result) *Recorder {
	r.fallback = result
	return r
}

// Install replaces the tmux command factory with one backed by this recorder
// and registers a cleanup to restore the previous factory when the test ends.
func (r *Recorder) Install(t *testing.T) *Recorder {
	t.Helper()
	restore := tmux.OverrideCommandFactory(func(client tmux.Client, args ...string) tmux.Commander {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.Calls = append(r.Calls, args)
		return &command{args: args, result: r.resultFor(args), recorder: r, index: len(r.Calls) - 1}
	})
	t.Cleanup(restore)
	return r
}

func (r *Recorder) resultFor(args []string) Result {
	sub := subcommandOf(args)
	if q := r.queues[sub]; len(q) > 0 {
		res := q[0]
		r.queues[sub] = q[1:]
		return res
	}
	return r.fallback
}

// Called reports whether the given tmux subcommand was ever invoked.
func (r *Recorder) Called(subcommand string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.Calls {
		if subcommandOf(c) == subcommand {
			return true
		}
	}
	return false
}

// CountOf returns how many times the given tmux subcommand was invoked.
func (r *Recorder) CountOf(subcommand string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, c := range r.Calls {
		if subcommandOf(c) == subcommand {
			count++
		}
	}
	return count
}

// ArgsFor returns the arguments of the first invocation of the given subcommand.
func (r *Recorder) ArgsFor(subcommand string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.Calls {
		if subcommandOf(c) == subcommand {
			return c
		}
	}
	return nil
}

// InputsFor returns the stdin of every invocation of the given subcommand, in call order.
func (r *Recorder) InputsFor(subcommand string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var inputs []string
	for i, c := range r.Calls {
		if subcommandOf(c) == subcommand {
			inputs = append(inputs, r.inputs[i])
		}
	}
	return inputs
}

// subcommandOf returns the tmux subcommand, skipping any leading socket flags
// (-L name / -S path) so routing works regardless of how args were assembled.
func subcommandOf(args []string) string {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-L", "-S":
			i++
			continue
		}
		return args[i]
	}
	return ""
}
