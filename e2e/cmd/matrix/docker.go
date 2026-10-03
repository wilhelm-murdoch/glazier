package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wilhelm-murdoch/glazier/e2e/report"
)

// caseTimeout bounds one target. A single case has its own deadlines, so this
// only stops a run that hangs outside them.
const caseTimeout = 30 * time.Minute

// tmuxRepository is where the tmuxnext target fetches tmux from.
const tmuxRepository = "https://github.com/tmux/tmux.git"

// testHeader is a line of go test -v that names the test of the next lines;
// testMessage is a line that a test logged.
var (
	testHeader  = regexp.MustCompile(`^=== (?:RUN|CONT|NAME|PAUSE)\s+(\S+)`)
	testMessage = regexp.MustCompile(`^\s+(\w+\.go:\d+): (.*)$`)
)

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
	tr := &targetRun{Target: target, Revision: rev.name}
	dir := filepath.Join(m.cfg.out, rev.name)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		tr.Err = err
		return tr
	}
	reportPath := filepath.Join(dir, target+".jsonl")
	tr.Log = filepath.Join(dir, target+".log")
	if err := os.Remove(reportPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		tr.Err = err
		return tr
	}
	logFile, err := os.Create(tr.Log)
	if err != nil {
		tr.Err = err
		return tr
	}
	defer func() { _ = logFile.Close() }()

	args := []string{
		"run", "--rm", "--init",
		"--platform", "linux/" + m.cfg.arch,
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
		"-test.count=1",
		"-test.v",
		"-test.parallel=" + strconv.Itoa(m.cfg.parallel),
		"-test.timeout=" + caseTimeout.String(),
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
		tr.Exit = exitErr.ExitCode()
	case err != nil:
		tr.Err = err
		return tr
	}

	tr.Records, err = report.ReadFile(reportPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		tr.Err = err
	}
	tr.FailedTests = failedTests(tr.Log)
	return tr
}

// failedTests reads the failed tests from a go test -v log, each with its
// first error line. A test can fail without a failed check, for example when
// a case stops with Fatal.
func failedTests(path string) []failedTest {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil
	}
	var (
		failed  []failedTest
		current string
		first   = map[string]string{}
	)
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if m := testHeader.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if name, ok := strings.CutPrefix(trimmed, "--- FAIL: "); ok {
			if i := strings.LastIndex(name, " ("); i > 0 {
				name = name[:i]
			}
			failed = append(failed, failedTest{Name: name, Message: first[name]})
			continue
		}
		// The harness logs each command as "file.go:N: $ command"; an error line has no "$ ".
		if m := testMessage.FindStringSubmatch(line); m != nil && current != "" && first[current] == "" && !strings.HasPrefix(m[2], "$ ") {
			first[current] = m[1] + ": " + m[2]
		}
	}
	return failed
}
