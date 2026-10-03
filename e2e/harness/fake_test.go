package harness

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// fakeT records what a Case reports, so a test can check a failing check
// without failing itself.
type fakeT struct {
	t        *testing.T
	mu       sync.Mutex
	errors   []string
	logs     []string
	fatal    string
	skipped  bool
	cleanups []func()
}

func (f *fakeT) Helper()         {}
func (f *fakeT) Name() string    { return "TestFake/case" }
func (f *fakeT) TempDir() string { return f.t.TempDir() }

func (f *fakeT) Cleanup(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanups = append(f.cleanups, fn)
}

func (f *fakeT) Logf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, fmt.Sprintf(format, args...))
}

func (f *fakeT) Errorf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func (f *fakeT) Fatal(args ...any) { f.Fatalf("%s", fmt.Sprint(args...)) }

func (f *fakeT) Fatalf(format string, args ...any) {
	f.mu.Lock()
	f.fatal = fmt.Sprintf(format, args...)
	f.mu.Unlock()
	runtime.Goexit()
}

func (f *fakeT) Skip(args ...any) {
	f.mu.Lock()
	f.skipped = true
	f.mu.Unlock()
	runtime.Goexit()
}

func (f *fakeT) errorText() string { return strings.Join(f.errors, "\n") }

// withCase runs fn with a Case on a fake T in its own goroutine, so that a
// Fatal stops fn only. It needs no tmux and no glaze.
func withCase(t *testing.T, env *Env, fn func(c *Case)) *fakeT {
	t.Helper()
	if env == nil {
		env = &Env{}
	}

	if env.Shell == "" {
		env.Shell = "/bin/sh"
	}

	f := &fakeT{t: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(newCase(f, env))
	}()

	<-done
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}

	return f
}
