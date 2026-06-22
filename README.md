# Glazier
_noun_ &middot; _/ˈɡleɪ.zi.ər/_ <sup>[pronounciation](https://www.google.com/search?q=pronounce+glazier)</sup>
> a person whose trade is fitting glass into windows and doors.
---

[![CI](https://github.com/wilhelm-murdoch/glazier/actions/workflows/ci.yaml/badge.svg)](https://github.com/wilhelm-murdoch/glazier/actions/workflows/ci.yaml)
[![GoDoc](https://godoc.org/github.com/wilhelm-murdoch/glazier?status.svg)](https://pkg.go.dev/github.com/wilhelm-murdoch/glazier)
[![Go Report Card](https://goreportcard.com/badge/github.com/wilhelm-murdoch/glazier)](https://goreportcard.com/report/github.com/wilhelm-murdoch/glazier)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/wilhelm-murdoch/glazier/badge)](https://scorecard.dev/viewer/?uri=github.com/wilhelm-murdoch/glazier)
[![Stability: Active](https://masterminds.github.io/stability/active.svg)](https://masterminds.github.io/stability/active.html)

`glaze` (Glazier) is a command-line tool for declaratively managing your tmux workspaces. Describe your sessions, windows, and panes once in an HCL `.glaze` file, then recreate them on demand. No more rebuilding layouts by hand after every reboot.

```hcl
session {
  name = "daemon-run"

  window {
    name   = "ice-breaker"
    layout = "main-vertical"

    pane {
      commands = ["nvim ./payloads"]
    }

    pane {
      commands = ["watch -n1 netwatch --target arasaka-mainframe"]
    }
  }
}
```

Simply type the following alongside a `.glaze` file.
```console
$ glaze up
```

### Why should I use this?

Honestly, only you can answer that. I originally built this for myself because I was interested in learning how Terraform parses and validates their HCL specs. I'm also a heavy tmux user, so these two things lined up perfectly. This has been a slow-burning labour of love for the past couple years and it's finally in a state where I feel comfortable sharing it with others.

There are plenty of other options out there - `teamocil`, `tmuxinator`, `smug`, etc... - that effectively do the same thing and have been around far longer. If you already use and trust any of these, there really isn't a strong value proposition for moving over to `glaze`; keep using them.

Personally, I like the declarative self-validating HCL spec, variable + string function support for templates, and being able to _mostly_ save a session. It's been an incredibly fun journey in over-engineering a solution to an already solved problem.

## Features
- HCL-based syntax with Terraform-style diagnostics.
- Multiple `*.glaze` definition files, resolved from flag, CWD, or `$GLAZE_PATH`.
- Arbitrary variable injection from `--var` flags and `GLAZE_ENV_*` env vars.
- Template functions for string manipulation.
- Environment variables, hooks, and tmux options.
- Reliable command sequencing via `tmux wait-for` (no fixed sleeps).
- Built-in formatting and validation (`glaze format`).
- Capture a live session back into a profile (`glaze save`).
- Tear down a profile's session (`glaze down`) and list what's running (`glaze ls`).

## Requirements
- Go **1.26+** (to build from source)
- `tmux` available on your `PATH`

## Installation

### From a GitHub release

Prebuilt, version-stamped binaries for linux/darwin on amd64/arm64 are
attached to every [GitHub release](https://github.com/wilhelm-murdoch/glazier/releases)
as zips, alongside a `SHA256SUMS` file and a signed
[build provenance attestation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations).

```console
$ unzip glaze-darwin-arm64.zip
$ shasum -a 256 -c SHA256SUMS --ignore-missing      # verify the checksum
$ gh attestation verify glaze-darwin-arm64.zip \
    --repo wilhelm-murdoch/glazier                  # verify it was built by this repo's release workflow
```

### From source
The following will build and install the `glaze` binary via `go install`.
```console
$ git clone https://github.com/wilhelm-murdoch/glazier.git
$ cd glazier
$ make install
```

### With `go install`
```console
$ go install github.com/wilhelm-murdoch/glazier/cmd/glaze@latest
```

### Build a local binary
Will compile and write the resulting binaries in `bin/<os>-amd64/glaze`.
```console
$ make build
```

> [!NOTE]
> At the moment only Linux and MacOS are supported operating systems.

## Usage
All subcommands have their own `--help` output.
```console
$ glaze --help
$ go run cmd/glaze/main.go --help
NAME:
   glaze - easily manage tmux sessions, windows and panes

USAGE:
   glaze [global options] [command [command options]]

VERSION:
   dev

AUTHOR:
   {Wilhelm Murdoch wilhelm@devilmayco.de}

COMMANDS:
   up       apply the specified glaze profile
   down     kill the session described by the specified glaze profile
   ls       list the sessions running on the target tmux server
   format   rewrites the target glaze profile file to a canonical format
   save     running this within a tmux session will save its current state to the specified glaze profile
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --log-level string  specify a log level (default: "info")
   --help, -h          show help
   --version, -v       print only the version

COPYRIGHT:
   (c) 2026 Wilhelm Codes ( https://wilhelm.codes )
```

Global flags:
| Flag | Default | Description |
|------|---------|-------------|
| `--log-level` | `info` | one of the supported log levels: `trace`, `debug`, `info`, `warning`, `error`, `critical` |

### `glaze up`
Apply a profile, creating the session, windows, and panes.
```console
$ glaze up                          # apply ./.glaze and jack in
$ glaze up --detached               # spin it up without jacking in
$ glaze up --clear                  # flatline an existing session of the same name first
$ glaze up --profile-path ./gig.glaze
$ glaze up --var district=watson --var fixer=wakako
```

| Flag | Description |
|------|-------------|
| `--detached` | create the session without attaching to it |
| `--clear` | kill an existing session with the same name before starting |
| `--debug` | print every command sent to the tmux socket |
| `--socket-path` | path to a custom tmux socket |
| `--socket-name` | name of a custom tmux socket |
| `--profile-path` | path to a `.glaze` file (see [Profile resolution](#profile-resolution)) |
| `--var key=value` | set a variable; repeatable |

### `glaze down`

Tear down the session a profile describes. The profile is resolved and decoded
exactly as it is for `up`, so an interpolated session name (e.g.
`name = "gig-${district}"`) resolves through the same `--var`/`GLAZE_ENV_*`
machinery. Bringing down a session that is not running is a no-op rather than
an error, so `down` stays idempotent for scripts.

```console
$ glaze down                        # kill the session described by ./.glaze
$ glaze down --var district=watson  # resolve an interpolated session name
$ glaze down --session daemon-run   # kill by name; no profile required
```

| Flag | Description |
|------|-------------|
| `--session` | session to kill (skips profile resolution entirely) |
| `--profile-path` | path to a `.glaze` file (see [Profile resolution](#profile-resolution)) |
| `--var key=value` | set a variable; repeatable |
| `--socket-path` / `--socket-name` | custom tmux socket |

### `glaze ls`

List the sessions running on the target tmux server, with window counts and
starting directories. When run from inside tmux, the session the current
client is attached to is marked with an asterisk.

```console
$ glaze ls
NAME         WINDOWS  PATH
daemon-run*  3        /home/v/runs/arasaka
scratch      1        /tmp
```

| Flag | Description |
|------|-------------|
| `--socket-path` / `--socket-name` | custom tmux socket |

### `glaze format`

Rewrite a profile into canonical HCL, optionally validating it first.

```console
$ glaze format                      # format ./.glaze in place
$ glaze format --stdout             # print formatted output instead of writing
$ glaze format --validate           # decode + report diagnostics, then format
```

### `glaze save`

Capture the current (or a named) running tmux session into a `.glaze` profile.

```console
$ glaze save                        # write ./.glaze from the current session
$ glaze save --stdout               # print the profile instead of writing
$ glaze save --session daemon-run --profile-path ./daemon-run.glaze
```

| Flag | Description |
|------|-------------|
| `--session` | session to capture (defaults to the current client's session) |
| `--profile-path` | output path (defaults to `.glaze`) |
| `--stdout` | print the profile instead of writing a file |
| `--socket-path` / `--socket-name` | custom tmux socket |

> [!NOTE]
> `save` captures the **structure** of a session: session, window and pane names, starting directories, focus (the active window and pane), and layout. Because tmux only reports a window's layout as a low-level coordinate string (e.g. `bb62,80x24,0,0`) rather than one of the named presets, `save` writes that raw string verbatim as a fallback. `glaze up` replays it exactly, so your pane geometry is restored even when it doesn't match a named preset. By design `save` does **not** export pane commands, environment variables, hooks, or tmux options.
>
> This is a deliberate safety choice, not a missing feature:
> - **Commands** (and **hooks**, which are commands bound to events) would re-execute on the next `glaze up` - a destructive command captured from a forgotten pane could nuke your filesystem or peg a database on replay.
> - **Environment variables** can only be read as the *entire* session environment, which includes secrets (tokens, keys) inherited from your shell - exporting them would write those into a file you might commit.
> - **Options** read back as effective state, mixing your `tmux.conf` and manual tweaks with anything glaze set, so re-applying them on `up` would be surprising and wrong.
>
> Treat a saved profile as a scaffold: it recreates your layout, and you add the commands, envs, and options you actually want by hand. Tip: a saved raw layout string is exact but not human-friendly - feel free to replace it with a named preset (`tiled`, `main-vertical`, etc...) for a profile you intend to hand-edit.

## Profile resolution

`glaze` locates a profile in this order:

- `--profile-path <path>` if provided
- `.glaze` in the current working directory
- `$GLAZE_PATH/.glaze`

> [!NOTE]
> `~` is expanded to your home directory in path values.

## Specification

A profile contains exactly one `session` block. Blocks take **no labels**; names are set with a `name` attribute. Strings, maps, and lists use standard HCL syntax.

### Session

```hcl
session {
  name               = "daemon-run"     # defaults to "default"
  starting_directory = "~/runs/arasaka" # defaults to the current directory

  envs = {
    EDITOR     = "nvim"
    ICE_TARGET = "arasaka-mainframe"
  }

  hooks = {
    "session-created" = "run-shell 'echo jacked-in'"
  }

  options = {
    "base-index" = "1"
  }

  window {
    # ...at least one window is required
  }
}
```

| Attribute | Type | Notes |
|-----------|------|-------|
| `name` | string | session name; defaults to `default` |
| `starting_directory` | string | must exist; defaults to CWD |
| `hooks` | map(string) | tmux hook name > command |
| `options` | map(string) | tmux option name > value |
| `window` | block(s) | one or more windows (required) |

### Window

```hcl
window {
  name   = "ice-breaker"
  layout = "main-vertical"   # even-horizontal | even-vertical | main-horizontal | main-vertical | tiled | a raw tmux layout string
  focus  = true              # make this the active window

  hooks   = { "window-renamed" = "display 'trace detected'" }
  options = { "automatic-rename" = "off" }

  pane {
    # ...at least one pane is required
  }
}
```

`layout` defaults to `tiled` when omitted. In addition to the five named presets it also accepts a **raw tmux layout coordinate string** (e.g. `"bb62,80x24,0,0"`) - this is what `glaze save` captures from a live window when no named preset applies, and `glaze up` replays it verbatim. The value is validated for structure when the profile is parsed; a malformed string fails fast. (tmux recomputes the leading checksum, so if you hand-edit the geometry and break it, tmux rejects the layout when `up` runs.) For hand-authored profiles, prefer a named preset - the raw string is exact but not human-readable.

### Pane

```hcl
pane {
  name     = "breach-protocol"
  focus    = true
  commands = ["nvim ./daemons", "echo upload ready"]

  size {                     # absolute resize
    x = "60%"                # cells (e.g. "80") or percentage (e.g. "60%")
    y = "100"
  }

  adjust {                   # directional resize, up to 4 blocks, applied in order
    direction = "left"       # up | down | left | right
    amount    = "5"
  }

  options = { "remain-on-exit" = "on" }
}
```

`commands` are sent in order and serialised with `tmux wait-for`, so each command finishes before the next is sent. The **final** command is sent fire-and-forget (no wait), so a long-running or interactive command (`nvim`, `tail -f`, a dev server) does not block session creation. `size` is applied first, then any `adjust` blocks refine the dimensions.

## Variables & string functions

Variables are referenced by name (e.g. `district`, `path.pwd`) and resolved (last write wins) from:

1. Environment variables prefixed with `GLAZE_ENV_` (e.g. `GLAZE_ENV_district=watson` > `district`).
2. `--var key=value` flags.
3. Built-in defaults: `path.pwd` (working directory) and `path.base` (its basename).

```hcl
session {
  name               = "gig-${district}"
  starting_directory = path.pwd

  window {
    name = upper(path.base)

    pane {
      commands = ["echo ${trimspace(fixer)} has the next job"]
    }
  }
}
```

```console
$ GLAZE_ENV_district=watson glaze up --var fixer="wakako"
```

Available functions (thin wrappers over `go-cty` stdlib):
- `replace`
- `regexreplace`
- `upper`
- `lower`
- `reverse`
- `len`
- `substr`
- `join`
- `title`
- `trim`
- `trimspace`
- `trimsuffix`
- `trimprefix`
- `chomp`

## Development

```console
$ go test ./...              # run the test suite (no extra tooling required)
$ go test -cover ./...       # with coverage
$ go vet ./...               # static analysis
$ go build ./...             # compile everything

$ make build                 # build bin/<os>-<arch>/glaze (version-stamped)
$ make test                  # run tests via gotestsum (pinned, via `go run`)
$ make race                  # tests under the race detector (needs CGO)
$ make cover                 # tests with coverage; enforces the 80% floor
$ make vet                   # go vet
$ make lint                  # golangci-lint incl. gosec (pinned, via `go run`)
$ make vuln                  # govulncheck vulnerability scan
$ make fuzz                  # native Go fuzzing, auto-discovers Fuzz* targets
$ make all                   # deps > build > test > race > lint > cover > vuln
$ make install               # go install the version-stamped binary
$ make release               # cross-compile + zip (linux/darwin, amd64/arm64)
```

Tooling versions are pinned in the `Makefile` and run with `go run <tool>@<version>`, so no global installs or `curl | sh` bootstrap scripts are needed. Linting is configured in [`.golangci.yml`](./.golangci.yml). Requires Go **1.26+** (the `Makefile` and CI both read the version from `go.mod`).

The test suite includes an end-to-end test (`pkg/tmux/e2e_test.go`) that drives a real `tmux` server on a throwaway socket; it self-skips when `tmux` is not on the `PATH`. The attacker-controllable surfaces (HCL profile decoding, variable collection) carry native Go fuzz targets whose seed corpora replay as plain tests in every build; `make fuzz` runs real input generation.

CI runs on [Woodpecker](./.woodpecker/workflow.yaml) and [GitHub Actions](./.github/workflows/) — both call the same Makefile targets, so a green local `make all` is a green build. GitHub additionally runs CodeQL, govulncheck, OpenSSF Scorecard, and weekly scheduled fuzzing.

See [CONTRIBUTING.md](./CONTRIBUTING.md) for conventions and [SECURITY.md](./SECURITY.md) for the security policy and reporting channel.

## AI Disclosure

The architecture, functionality and base structure of this project are my own. I use AI as a tool to assist with time-consuming work - documentation, tests, and bug hunting - and as a sounding board for structural decisions that keep the project easy to adopt. For a solo developer it's a force multiplier for shipping high-quality code efficiently; simply a tool to address drudgery and toil, not a crutch.

## License

[MIT](LICENSE) © Wilhelm Murdoch
