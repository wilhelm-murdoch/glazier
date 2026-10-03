// Command matrix runs the end-to-end cases on every tmux target in Docker.
//
// It builds one image for each target, cross-compiles glaze and the test
// binary for Linux, runs every target in parallel and prints a summary. With
// -base, it also builds glaze from a base revision and compares the two.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
)

// defaultParallel is the number of cases that run at the same time on one
// target. All targets run at the same time, so keep it low.
var defaultParallel = max(2, runtime.NumCPU()/4)

// allTargets are the stages of images/Dockerfile that run by default, in the
// order of the summary.
var allTargets = []string{"bookworm", "trixie", "jammy", "noble", "alpine", "tmux37"}

// extraTargets run only when -targets names them. tmuxnext builds the
// development branch of tmux, for the weekly canary.
var extraTargets = []string{"tmuxnext"}

// config is the parsed command line.
type config struct {
	targets       []string
	arch          string
	base          string
	glaze         string
	baseGlaze     string
	run           string
	parallel      int
	expectVersion string
	out           string
	markdown      string
	skipImages    bool
	tmuxRef       string
	count         int
	cpus          string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "matrix:", err)
		var failed *failedError
		if errors.As(err, &failed) {
			os.Exit(1)
		}

		os.Exit(2)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := parseFlags(args)
	if err != nil {
		return err
	}

	repo, err := gitOutput(ctx, "", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("find the repository: %w", err)
	}

	m := &matrix{cfg: cfg, repo: repo, root: filepath.Join(repo, "e2e")}
	if cfg.out == "" {
		cfg.out = filepath.Join(m.root, "results")
	}

	if cfg.arch == "" {
		if cfg.arch, err = dockerArch(ctx); err != nil {
			return err
		}
	}

	return m.run(ctx)
}

func parseFlags(args []string) (*config, error) {
	cfg := &config{}
	fs := flag.NewFlagSet("matrix", flag.ContinueOnError)
	targets := fs.String("targets", strings.Join(allTargets, ","), "comma-separated targets: "+strings.Join(allTargets, ", "))
	fs.StringVar(&cfg.arch, "arch", "", "the architecture of the images and binaries (default: that of the Docker server)")
	fs.StringVar(&cfg.base, "base", "", "a git revision to compare with; glaze is built from it and runs the cases of the working tree")
	fs.StringVar(&cfg.glaze, "glaze", "", "a Linux glaze binary to test instead of a build of the working tree, for example a release")
	fs.StringVar(&cfg.baseGlaze, "base-glaze", "", "a Linux glaze binary to compare with instead of a build of -base")
	fs.StringVar(&cfg.run, "run", "", "run only the cases that match this go test -run pattern")
	fs.IntVar(&cfg.parallel, "parallel", defaultParallel, "the number of cases that run at the same time on one target")
	fs.StringVar(&cfg.expectVersion, "expect-version", "", "the version that glaze --version must print")
	fs.StringVar(&cfg.out, "out", "", "the directory for reports, logs and binaries (default: e2e/results)")
	fs.StringVar(&cfg.markdown, "markdown", "", "also write the summary as markdown to this file")
	fs.BoolVar(&cfg.skipImages, "skip-images", false, "use the images that exist; do not build them")
	fs.StringVar(&cfg.tmuxRef, "tmux-ref", "master", "the branch or tag of tmux that the tmuxnext target builds")
	fs.IntVar(&cfg.count, "count", 1, "run each case this many times, to find a flaky check")
	fs.StringVar(&cfg.cpus, "cpus", "", "limit each container to this many CPUs (docker run --cpus), to find a flaky check")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	for _, t := range strings.Split(*targets, ",") {
		if t = strings.TrimSpace(t); t == "" {
			continue
		}

		if !slices.Contains(allTargets, t) && !slices.Contains(extraTargets, t) {
			return nil, fmt.Errorf("unknown target %q; the targets are %s", t, strings.Join(append(slices.Clone(allTargets), extraTargets...), ", "))
		}

		cfg.targets = append(cfg.targets, t)
	}

	if len(cfg.targets) == 0 {
		return nil, errors.New("no target")
	}

	if cfg.baseGlaze != "" && cfg.base == "" {
		cfg.base = "base"
	}

	if cfg.parallel < 1 || cfg.count < 1 {
		return nil, errors.New("-parallel and -count must be 1 or more")
	}

	return cfg, nil
}
