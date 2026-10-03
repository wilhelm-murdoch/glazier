package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// stamp is what glaze --version prints, set with -ldflags -X like the Makefile does.
type stamp struct {
	version string
	commit  string
}

// buildGlaze puts the glaze binary of a revision into m.bin: a copy of -glaze
// or -base-glaze, a build of the working tree or a build of -base.
func (m *matrix) buildGlaze(ctx context.Context, rev revision) error {
	given := m.cfg.glaze
	if rev.name == baseRevision {
		given = m.cfg.baseGlaze
	}

	if given != "" {
		return copyFile(given, rev.glaze)
	}

	if rev.name == headRevision {
		st, err := m.stampOf(ctx, "HEAD", true)
		if err != nil {
			return err
		}

		return m.goBuild(ctx, m.repo, rev.glaze, st)
	}

	src, err := os.MkdirTemp("", "glaze-base-")
	if err != nil {
		return err
	}

	defer func() { _ = os.RemoveAll(src) }()
	if err := m.export(ctx, m.cfg.base, src); err != nil {
		return err
	}

	st, err := m.stampOf(ctx, m.cfg.base, false)
	if err != nil {
		return err
	}

	return m.goBuild(ctx, src, rev.glaze, st)
}

// buildTests compiles the cases into one static Linux test binary, so the
// images need no Go toolchain.
func (m *matrix) buildTests(ctx context.Context, out string) error {
	cmd := exec.CommandContext(ctx, "go", "test", "-c", "-o", out, "./cases") // #nosec G204 -- fixed arguments
	cmd.Dir = m.root
	cmd.Env = m.goEnv()
	return runLogged(cmd)
}

func (m *matrix) goBuild(ctx context.Context, src, out string, st stamp) error {
	ldflags := strings.Join([]string{
		"-s", "-w",
		"-X", "main.Version=" + st.version,
		"-X", "main.Commit=" + st.commit,
		"-X", "main.Date=" + time.Now().Format("2006-01-02T15:04:05-0700"),
		"-X", "main.Stage=e2e",
	}, " ")
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", ldflags, "-o", out, "./cmd/glaze") // #nosec G204 -- the version comes from git
	cmd.Dir = src
	cmd.Env = m.goEnv()
	return runLogged(cmd)
}

func (m *matrix) goEnv() []string {
	return append(os.Environ(), "GOOS=linux", "GOARCH="+m.cfg.arch, "CGO_ENABLED=0", "GOFLAGS=-mod=readonly")
}

// stampOf reads the version and the commit of a revision from git.
func (m *matrix) stampOf(ctx context.Context, ref string, dirty bool) (stamp, error) {
	args := []string{"describe", "--tags"}
	if dirty {
		args = append(args, "--dirty")
	} else {
		args = append(args, ref)
	}

	version, err := gitOutput(ctx, m.repo, args...)
	if err != nil {
		version = "dev"
	}

	commit, err := gitOutput(ctx, m.repo, "rev-parse", "--short", ref)
	if err != nil {
		return stamp{}, fmt.Errorf("resolve %s: %w", ref, err)
	}

	return stamp{version: version, commit: commit}, nil
}

// export writes the tree of a revision to dir with git archive.
func (m *matrix) export(ctx context.Context, ref, dir string) error {
	archive := exec.CommandContext(ctx, "git", "archive", "--format=tar", ref) // #nosec G204 -- a revision that the user gives
	archive.Dir = m.repo
	untar := exec.CommandContext(ctx, "tar", "-x", "-C", dir) // #nosec G204 -- a temporary directory
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return err
	}

	untar.Stdin = pipe
	archive.Stderr, untar.Stderr = os.Stderr, os.Stderr
	if err := untar.Start(); err != nil {
		return err
	}

	if err := archive.Run(); err != nil {
		return fmt.Errorf("git archive %s: %w", ref, err)
	}

	return untar.Wait()
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- git with arguments from the runner
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func runLogged(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", strings.Join(cmd.Args, " "), err, out)
	}

	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(filepath.Clean(from))
	if err != nil {
		return err
	}

	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(filepath.Clean(to), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) // #nosec G302 -- an executable for the containers
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}

	return out.Close()
}
