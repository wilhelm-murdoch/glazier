package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// The two revisions that a run can test. Head is the working tree or -glaze;
// base is -base or -base-glaze.
const (
	headRevision = "head"
	baseRevision = "base"
)

const (
	dirMode  = 0o755
	fileMode = 0o644
)

// matrix is one run of the runner.
type matrix struct {
	cfg  *config
	repo string // the root of the repository
	root string // the root of the e2e module
	bin  string // the directory for the binaries that the containers mount
}

// revision is one glaze binary under test.
type revision struct {
	name  string // headRevision or baseRevision
	label string // what the summary shows, for example a git revision
	glaze string // the path of the binary in m.bin
}

// failedError means that the cases ran and at least one failed.
type failedError struct{ msg string }

func (e *failedError) Error() string { return e.msg }

// run builds everything, runs every revision on every target and prints the summary.
func (m *matrix) run(ctx context.Context) error {
	m.bin = filepath.Join(m.cfg.out, "bin")
	if err := os.MkdirAll(m.bin, dirMode); err != nil {
		return err
	}

	revisions := m.revisions()

	logf("arch %s, targets %v", m.cfg.arch, m.cfg.targets)
	if err := m.prepare(ctx, revisions); err != nil {
		return err
	}

	runs := m.runAll(ctx, revisions)
	if err := ctx.Err(); err != nil {
		return err
	}

	s := newSummary(m.cfg.targets, revisions, runs)
	if err := m.writeSummary(s.markdown()); err != nil {
		return err
	}

	if msg := s.problem(); msg != "" {
		return &failedError{msg: msg}
	}

	return nil
}

// revisions returns the glaze binaries to test: head, and base before it when
// the run compares.
func (m *matrix) revisions() []revision {
	head := revision{name: headRevision, label: "working tree", glaze: filepath.Join(m.bin, "glaze-head")}
	if m.cfg.glaze != "" {
		head.label = filepath.Base(m.cfg.glaze)
	}

	if m.cfg.base == "" {
		return []revision{head}
	}

	base := revision{name: baseRevision, label: m.cfg.base, glaze: filepath.Join(m.bin, "glaze-base")}
	if m.cfg.baseGlaze != "" {
		base.label = filepath.Base(m.cfg.baseGlaze)
	}

	return []revision{base, head}
}

// runAll runs every revision on every target at the same time.
func (m *matrix) runAll(ctx context.Context, revisions []revision) []*targetRun {
	versions := m.tmuxVersions(ctx)
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		runs []*targetRun
	)
	for _, rev := range revisions {
		for _, target := range m.cfg.targets {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tr := m.runTarget(ctx, rev, target)
				tr.tmux = versions[target]
				logf("%-4s %-9s %s", rev.name, target, tr.brief())
				mu.Lock()
				runs = append(runs, tr)
				mu.Unlock()
			}()
		}
	}

	wg.Wait()
	return runs
}

// writeSummary prints the summary and writes it to summary.md and to -markdown.
func (m *matrix) writeSummary(md string) error {
	fmt.Print(md)
	paths := []string{filepath.Join(m.cfg.out, "summary.md")}
	if m.cfg.markdown != "" {
		paths = append(paths, m.cfg.markdown)
	}

	for _, path := range paths {
		if err := os.WriteFile(path, []byte(md), fileMode); err != nil { // #nosec G306 -- a report for the user
			return err
		}
	}

	return nil
}

// prepare builds the images, the glaze binaries and the test binary at the
// same time.
func (m *matrix) prepare(ctx context.Context, revisions []revision) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	do := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				mu.Unlock()
			}
		}()
	}

	if !m.cfg.skipImages {
		for _, target := range m.cfg.targets {
			do("image "+target, func() error { return m.buildImage(ctx, target) })
		}
	}

	for _, rev := range revisions {
		do("glaze "+rev.name, func() error { return m.buildGlaze(ctx, rev) })
	}

	do("test binary", func() error { return m.buildTests(ctx, filepath.Join(m.bin, "e2e.test")) })
	wg.Wait()
	if len(errs) > 0 {
		return fmt.Errorf("prepare: %v", errs)
	}

	return nil
}

func (m *matrix) tmuxVersions(ctx context.Context) map[string]string {
	versions := map[string]string{}
	for _, target := range m.cfg.targets {
		versions[target] = m.tmuxVersion(ctx, target)
	}

	return versions
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "matrix: "+format+"\n", args...)
}
