// Package harness runs the glaze binary against a real tmux server and checks
// the result through tmux itself. It never imports glaze, so every check stays
// independent of the code under test.
package harness

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	update = flag.Bool("update", false, "rewrite the golden files from the actual output")

	errNoGlaze = errors.New("GLAZE_BIN is not set")

	sharedEnv *Env
	envErr    error
	envOnce   sync.Once
)

// Env is what every case shares: the binaries under test, the target and the
// files of the module.
type Env struct {
	Glaze         string // absolute path of the glaze binary under test
	Tmux          string // absolute path of the tmux binary
	TmuxVersion   string // the version that tmux -V prints, for example "3.5a"
	Target        string // E2E_TARGET, or "host"
	ExpectVersion string // E2E_EXPECT_VERSION: the version that glaze --version must print
	Shell         string // the SHELL of every process, so the default-shell of tmux
	Root          string // the root of the e2e module, which holds fixtures and golden

	baseline *Baseline
	report   *reporter
}

// Main runs the cases of a package. Without GLAZE_BIN every case skips, unless
// E2E_REQUIRE is set, which turns a missing binary into a failure.
func Main(m *testing.M) int {
	flag.Parse()
	env, err := Load()
	if err != nil && !errors.Is(err, errNoGlaze) {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		return 2
	}
	if env != nil {
		fmt.Fprintf(os.Stderr, "e2e: target %s, tmux %s, glaze %s\n", env.Target, env.TmuxVersion, env.Glaze)
	}
	code := m.Run()
	if env != nil {
		if err := env.report.close(); err != nil {
			fmt.Fprintln(os.Stderr, "e2e: report:", err)
			return 1
		}
	}
	return code
}

// Load reads the environment once.
func Load() (*Env, error) {
	envOnce.Do(func() { sharedEnv, envErr = load() })
	return sharedEnv, envErr
}

func load() (*Env, error) {
	root, err := findRoot()
	if err != nil {
		return nil, err
	}
	glaze := os.Getenv("GLAZE_BIN")
	if glaze == "" {
		return nil, errNoGlaze
	}
	if glaze, err = absExecutable(glaze); err != nil {
		return nil, fmt.Errorf("GLAZE_BIN: %w", err)
	}
	tmux, err := absExecutable(getenv("TMUX_BIN", "tmux"))
	if err != nil {
		return nil, fmt.Errorf("TMUX_BIN: %w", err)
	}
	out, err := exec.Command(tmux, "-V").Output() // #nosec G204 -- the tmux binary under test
	if err != nil {
		return nil, fmt.Errorf("%s -V: %w", tmux, err)
	}
	shell, err := absExecutable(getenv("E2E_SHELL", "bash"))
	if err != nil {
		shell = "/bin/sh"
	}
	target := getenv("E2E_TARGET", "host")
	baseline, err := LoadBaseline(filepath.Join(root, "expected-failures.txt"), target)
	if err != nil {
		return nil, err
	}
	report, err := openReport(os.Getenv("E2E_REPORT"), target)
	if err != nil {
		return nil, err
	}
	return &Env{
		Glaze:         glaze,
		Tmux:          tmux,
		TmuxVersion:   strings.TrimPrefix(strings.TrimSpace(string(out)), "tmux "),
		Target:        target,
		ExpectVersion: os.Getenv("E2E_EXPECT_VERSION"),
		Shell:         shell,
		Root:          root,
		baseline:      baseline,
		report:        report,
	}, nil
}

// findRoot walks up from the working directory to the directory that holds
// the fixtures, so a test binary runs from any directory inside the module.
func findRoot() (string, error) {
	if root := os.Getenv("E2E_ROOT"); root != "" {
		return filepath.Abs(root)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isDir(filepath.Join(dir, "fixtures")) && isFile(filepath.Join(dir, "go.mod")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("cannot find the e2e module root; set E2E_ROOT")
		}
		dir = parent
	}
}

func absExecutable(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
