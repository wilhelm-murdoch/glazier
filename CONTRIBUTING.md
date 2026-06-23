# Contributing

Thanks for your interest in improving glazier!

glaze is a CLI for declaratively managing tmux workspaces from HCL `.glaze`
profiles. In-scope contributions: profile syntax, the tmux client library
(`pkg/tmux`), CLI commands, diagnostics quality, and docs. Out of scope:
anything that belongs in your `tmux.conf` (key bindings, status line theming)
or support for multiplexers other than tmux.

## Getting started

You'll need **Go 1.26+** and `tmux` on your `PATH` (the end-to-end test
self-skips without it, but you want it running). Clone the repository and,
from its root:

```sh
git clone https://github.com/wilhelm-murdoch/glazier.git
cd glazier
```

```sh
make test    # unit tests via gotestsum (pinned via go run, no global install)
make race    # unit tests under the race detector (needs CGO)
make lint    # golangci-lint incl. gosec
make cover   # unit tests with coverage; enforces the 80% floor
make vuln    # govulncheck vulnerability scan
make fuzz    # native Go fuzzing, auto-discovers Fuzz* targets (FUZZTIME=10s each)
make vet     # go vet
make fmt     # gofmt the tree
make build   # version-stamped binary in bin/<os>-<arch>/glaze
```

CI calls these exact Makefile targets, so a green local run is a green build -
no separate install steps, the pinned tool versions download on first use. CI
additionally runs the tests on macOS, on both the minimum supported Go version
and the latest stable release.

## How the code is organized

- `cmd/glaze/main.go` - CLI wiring via `urfave/cli/v3`; one entry per subcommand.
- `cmd/glaze/actions/` - one file per subcommand (`up`, `format`, `save`);
  `base.go` holds the shared profile-resolution/diagnostics scaffolding.
- `internal/spec/` - the `hcldec` specification tree for the profile schema.
- `internal/decoders/` - decode the raw `cty.Value` into typed `Session`/
  `Window`/`Pane` structs. New profile attributes touch spec + decoder + tests.
- `internal/parser/` - hclparse wrapper plus variable collection (`--var`,
  `GLAZE_ENV_*`) and the template function table.
- `internal/diagnostics/` - Terraform-style diagnostic rendering and the
  custom validators (layout strings, directories, sizes).
- `pkg/tmux/` - the tmux client library: every tmux invocation goes through
  the `Commander` interface in `command.go`, which is the dependency-injection
  seam tests use (`tmuxtest.Recorder`). `e2e_test.go` drives a real tmux
  server on a throwaway socket.
- `pkg/files/` - profile path resolution and `~` expansion.

`AGENTS.md` is the long-form reference for conventions, data flow, and the
reasoning behind deliberate omissions (notably what `save` refuses to export).

## Adding or changing behavior

1. New behavior needs a test. Fakes go through `tmuxtest.Recorder` /
   `OverrideCommandFactory`; never spawn tmux in unit tests.
2. Anything touching shared mutable state must stay safe under `-race`; run
   `make race`.
3. User-facing errors are HCL diagnostics where a profile is involved -
   extend the validators in `internal/diagnostics` rather than returning
   bare errors.
4. New profile attributes need: spec entry, decoder field, validator (if
   constrained), README documentation, and tests at each layer.

## Pull requests

- Branch off `develop` and name it `<type>/<description>` (e.g.
  `fix/15-glaze-down-decode`). See [RELEASE.md](RELEASE.md) for the
  branching model, naming convention, and how releases are cut.
- Keep changes focused; separate unrelated work into separate PRs.
- Write clear, imperative commit messages explaining the *why*.
- Make sure `make test`, `make race`, `make lint`, and `make vet` pass.
- New behavior needs tests.

By contributing, you agree that your work is licensed under the project's [MIT License](LICENSE.md).
