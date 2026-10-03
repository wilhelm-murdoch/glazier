package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/report"
)

// targetTimeout bounds the run of one target. Each case has its own deadlines,
// so this only stops a run that hangs outside them.
const targetTimeout = 30 * time.Minute

// tmuxRepository is where the tmuxnext target fetches tmux from.
const tmuxRepository = "https://github.com/tmux/tmux.git"

// The paths inside a container.
const (
	containerRoot    = "/e2e"
	containerBin     = "/opt/e2e/bin"
	containerResults = "/results"
)

func dockerArch(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Arch}}").Output()
	if err != nil {
		return "", fmt.Errorf("docker version: %w; is Docker running?", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// cpus is the CPU limit of a container; 0 means no limit.
func (m *matrix) cpus() string {
	if m.cfg.cpus == "" {
		return "0"
	}

	return m.cfg.cpus
}

func (m *matrix) image(target string) string { return "glaze-e2e:" + target + "-" + m.cfg.arch }

func (m *matrix) buildImage(ctx context.Context, target string) error {
	dir := filepath.Join(m.root, "images")
	args := []string{"build", "-q", "--platform", "linux/" + m.cfg.arch, "--target", target, "-t", m.image(target)}
	if target == "tmuxnext" {
		commit, err := tmuxCommit(ctx, m.cfg.tmuxRef)
		if err != nil {
			return err
		}

		logf("tmuxnext builds tmux %s at %s", m.cfg.tmuxRef, commit)
		args = append(args, "--build-arg", "TMUX_COMMIT="+commit)
	}

	cmd := exec.CommandContext(ctx, "docker", append(args, "-f", filepath.Join(dir, "Dockerfile"), dir)...) // #nosec G204 -- a known target
	return runLogged(cmd)
}

// tmuxCommit resolves a branch or tag of tmux to a commit, so that the image
// cache keeps one image for each commit.
func tmuxCommit(ctx context.Context, ref string) (string, error) {
	out, err := gitOutput(ctx, "", "ls-remote", tmuxRepository, ref)
	if err != nil {
		return "", fmt.Errorf("resolve tmux %s: %w", ref, err)
	}

	commit, _, _ := strings.Cut(out, "\t")
	if len(commit) != 40 {
		return "", fmt.Errorf("resolve tmux %s: no such branch or tag", ref)
	}

	return commit, nil
}

func (m *matrix) tmuxVersion(ctx context.Context, target string) string {
	out, err := exec.CommandContext(ctx, "docker", "run", "--rm", "--platform", "linux/"+m.cfg.arch, m.image(target), "tmux", "-V").Output() // #nosec G204 -- a known target
	if err != nil {
		return "?"
	}

	return strings.TrimPrefix(strings.TrimSpace(string(out)), "tmux ")
}

// runTarget runs the test binary for one revision on one target. The report
// and the log go to <out>/<revision>/<target>.{jsonl,log}.
func (m *matrix) runTarget(ctx context.Context, rev revision, target string) *targetRun {
	tr := &targetRun{target: target, revision: rev.name}
	dir := filepath.Join(m.cfg.out, rev.name)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		tr.err = err
		return tr
	}

	reportPath := filepath.Join(dir, target+".jsonl")
	tr.log = filepath.Join(dir, target+".log")
	if err := os.Remove(reportPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		tr.err = err
		return tr
	}

	logFile, err := os.Create(tr.log)
	if err != nil {
		tr.err = err
		return tr
	}

	defer func() { _ = logFile.Close() }()

	args := []string{
		"run", "--rm", "--init",
		"--platform", "linux/" + m.cfg.arch,
		"--cpus", m.cpus(),
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-v", m.root + ":" + containerRoot + ":ro",
		"-v", m.bin + ":" + containerBin + ":ro",
		"-v", dir + ":" + containerResults,
		"-e", "GLAZE_BIN=" + containerBin + "/" + filepath.Base(rev.glaze),
		"-e", "E2E_ROOT=" + containerRoot,
		"-e", "E2E_TARGET=" + target,
		"-e", "E2E_REPORT=" + containerResults + "/" + target + ".jsonl",
		"-e", "E2E_REQUIRE=1",
		"-e", "E2E_EXPECT_VERSION=" + m.cfg.expectVersion,
		"-w", containerRoot + "/cases",
		m.image(target),
		containerBin + "/e2e.test",
		"-test.count=" + strconv.Itoa(m.cfg.count),
		"-test.v",
		"-test.parallel=" + strconv.Itoa(m.cfg.parallel),
		"-test.timeout=" + targetTimeout.String(),
	}

	if m.cfg.run != "" {
		args = append(args, "-test.run="+m.cfg.run)
	}

	cmd := exec.CommandContext(ctx, "docker", args...) // #nosec G204 -- the runner builds the arguments
	cmd.Stdout, cmd.Stderr = logFile, logFile
	err = cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		tr.exit = exitErr.ExitCode()
	case err != nil:
		tr.err = err
		return tr
	}

	tr.records, err = report.ReadFile(reportPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		tr.err = err
	}

	tr.failedTests = parseFailedTests(tr.log)
	return tr
}
