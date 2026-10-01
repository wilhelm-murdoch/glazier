# Glazier
_noun_ &middot; _/ˈɡleɪ.zi.ər/_ <sup>[pronunciation](https://www.google.com/search?q=pronounce+glazier)</sup>
> a person whose trade is fitting glass into windows and doors.
---

[![CI](https://github.com/wilhelm-murdoch/glazier/actions/workflows/ci.yaml/badge.svg)](https://github.com/wilhelm-murdoch/glazier/actions/workflows/ci.yaml)
[![GoDoc](https://godoc.org/github.com/wilhelm-murdoch/glazier?status.svg)](https://pkg.go.dev/github.com/wilhelm-murdoch/glazier)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/wilhelm-murdoch/glazier/badge)](https://scorecard.dev/viewer/?uri=github.com/wilhelm-murdoch/glazier)
[![Stability: Active](https://masterminds.github.io/stability/active.svg)](https://masterminds.github.io/stability/active.html)

`glaze` (Glazier) is a command-line tool for declaratively managing your tmux workspaces. Describe your sessions, windows and panes once in an HCL `.glaze` file, then recreate them on demand. No more rebuilding layouts by hand after every reboot.

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

Type this command in a directory that contains a `.glaze` file.
```console
$ glaze up
```

### Why should I use this?

Honestly, only you can answer that. I originally built this for myself because I was interested in learning how Terraform parses and validates their HCL specs. I'm also a heavy tmux user, so these two things lined up perfectly. This has been a slow-burning labour of love for the past couple years and it's finally in a state where I feel comfortable sharing it with others.

There are plenty of other options out there - `teamocil`, `tmuxinator`, `smug`, etc... - that effectively do the same thing and have been around far longer. If you already use and trust any of these, there really isn't a strong value proposition for moving over to `glaze`; keep using them.

Personally, I like the declarative self-validating HCL spec, variable + string function support for templates and being able to _mostly_ save a session. It's been an incredibly fun journey in over-engineering a solution to an already solved problem.

## Features
- The syntax is HCL with Terraform-style diagnostics.
- Glazier supports multiple `*.glaze` files. It resolves them from a flag, the current directory or `$GLAZE_PATH`.
- Profiles declare typed `variable` blocks in the Terraform style. You read them through `var.`. Built-in `GLAZE_ENV_*` variables are also available.
- Template functions give string manipulation.
- Profiles set environment variables, hooks and tmux options.
- `tmux wait-for` sequences the commands. There are no fixed sleeps.
- The `glaze format` command formats and validates a profile.
- The `glaze save` command captures a live session into a profile.
- The `glaze down` command kills a profile's session. The `glaze ls` command lists the sessions.

## Requirements
- Go **1.26+** (to build from source)
- `tmux` available on your `PATH`

## Installation

### From a GitHub release

Each [GitHub release](https://github.com/wilhelm-murdoch/glazier/releases) includes prebuilt binaries for Linux and macOS on amd64 and arm64. The binaries are version-stamped and packaged as zips. Each release also includes a `SHA256SUMS` file and a signed [build provenance attestation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations).

```console
$ unzip glaze-darwin-arm64.zip
$ shasum -a 256 -c SHA256SUMS --ignore-missing      # verify the checksum
$ gh attestation verify glaze-darwin-arm64.zip \
    --repo wilhelm-murdoch/glazier                  # verify that this repo's release workflow built the zip
```

### From source
These commands build and install the `glaze` binary with `go install`.
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
This command compiles the binaries and writes them to `bin/<os>-amd64/glaze`.
```console
$ make build
```

> [!NOTE]
> Glazier supports only Linux and macOS.

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
| `--log-level` | `info` | One of the supported log levels: `trace`, `debug`, `info`, `warning`, `error`, `critical`. |

### `glaze up`
Apply a profile. The command creates the session, the windows and the panes.
```console
$ glaze up                          # apply ./.glaze and attach
$ glaze up --detached               # create the session and do not attach
$ glaze up --clear                  # first kill an existing session with the same name
$ glaze up --profile-path ./gig.glaze
$ glaze up --var district=watson --var fixer=wakako
```

| Flag | Description |
|------|-------------|
| `--detached` | Create the session and do not attach to it. |
| `--clear` | First kill an existing session that has the same name. |
| `--debug` | Print each command that Glazier sends to the tmux socket. |
| `--socket-path` | The path to a custom tmux socket. |
| `--socket-name` | The name of a custom tmux socket. |
| `--profile-path` | The path to a `.glaze` file. See [Profile resolution](#profile-resolution). |
| `--var key=value` | Set a variable. The flag is repeatable. |
| `--var-file <path>` | An HCL file of variable values. |

### `glaze down`

Tear down the session that a profile describes. Glazier evaluates only the session `name`. An interpolated name, for example `name = "gig-${var.district}"`, resolves through the same `--var` flags as `up`. A variable that appears only deeper in the profile is not required. A session that is not running causes no error. Thus `down` stays idempotent for scripts.

```console
$ glaze down                        # kill the session that ./.glaze describes
$ glaze down --var district=watson  # resolve an interpolated session name
$ glaze down --session daemon-run   # kill by name; no profile is required
```

| Flag | Description |
|------|-------------|
| `--session` | The session to kill. The flag skips profile resolution. |
| `--profile-path` | The path to a `.glaze` file. See [Profile resolution](#profile-resolution). |
| `--var key=value` | Set a variable. The flag is repeatable. |
| `--var-file <path>` | An HCL file of variable values. |
| `--socket-path` / `--socket-name` | A custom tmux socket. |

### `glaze ls`

List the sessions on the target tmux server with window counts and starting directories. When you run the command inside tmux, Glazier marks the attached session with an asterisk.

```console
$ glaze ls
NAME         WINDOWS  PATH
daemon-run*  3        /home/v/runs/arasaka
scratch      1        /tmp
```

| Flag | Description |
|------|-------------|
| `--socket-path` / `--socket-name` | A custom tmux socket. |

### `glaze format`

Rewrite a profile into canonical HCL. The `--validate` flag validates the profile first.

```console
$ glaze format                                   # format ./.glaze in place
$ glaze format --stdout                          # print the formatted output; do not write
$ glaze format --validate                        # decode and report diagnostics, then format
$ glaze format --validate --var region=us-east-1 # supply declared variables
```

| Flag | Description |
|------|-------------|
| `--stdout` | Print the formatted output. Do not write the file. |
| `--validate` | Decode the profile and report diagnostics before the format step. |
| `--profile-path` | The path to a `.glaze` file. See [Profile resolution](#profile-resolution). |
| `--var key=value` | Set a variable. The flag is repeatable. |
| `--var-file <path>` | An HCL file of variable values. |

The `--validate` flag enforces the full variable contract. A required variable must get a value from `--var` or from a default. A profile fails validation without one.

### `glaze save`

Capture the current running tmux session, or a named session, into a `.glaze` profile.

```console
$ glaze save                        # write ./.glaze from the current session
$ glaze save --stdout               # print the profile; do not write
$ glaze save --session daemon-run --profile-path ./daemon-run.glaze
```

| Flag | Description |
|------|-------------|
| `--session` | The session to capture. The default is the current client's session. |
| `--profile-path` | The output path. The default is `.glaze`. |
| `--stdout` | Print the profile. Do not write a file. |
| `--socket-path` / `--socket-name` | A custom tmux socket. |

> [!NOTE]
> `save` captures the **structure** of a session: the session, window and pane names, the starting directories, the focus and the layout. tmux reports a window layout only as a low-level coordinate string, for example `bb62,80x24,0,0`. It does not report a named preset. The `save` command writes that raw string verbatim as a fallback. The `up` command replays it exactly. Thus Glazier restores your pane geometry even when it does not match a named preset. By design `save` does **not** export pane commands, environment variables, hooks or tmux options.
>
> This is a deliberate safety choice, not a missing feature:
> - An exported **command** runs again on the next `glaze up`. A **hook** is a command bound to an event, thus the same risk applies. A destructive command from a forgotten pane can delete your filesystem or overload a database on replay.
> - Glazier can read **environment variables** only as the full session environment. That environment includes secrets from your shell, for example tokens and keys. An export writes those secrets into a file that you could commit.
> - **Options** read back as effective state. They mix your `tmux.conf` and your manual changes with the values that glaze set. To apply that state again on `up` gives unwanted results.
>
> Treat a saved profile as a scaffold. It recreates your layout. You add the commands, the environment variables and the options by hand. A saved raw layout string is exact but not easy to read. You can replace it with a named preset, for example `tiled` or `main-vertical`, in a profile that you edit by hand.

## Profile resolution

`glaze` locates a profile in this order:

- the `--profile-path <path>` value, if you supply it
- `.glaze` in the current working directory
- `$GLAZE_PATH/.glaze`

> [!NOTE]
> Glazier expands `~` to your home directory in path values.

## Specification

A profile contains exactly one `session` block. Blocks have **no labels**. You set a name with the `name` attribute. Strings, maps and lists use standard HCL syntax.

### Session

```hcl
session {
  name               = "daemon-run"     # the default value is "default"
  starting_directory = "~/runs/arasaka" # the default value is the current directory

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
| `name` | string | The session name. The default value is `default`. |
| `starting_directory` | string | The directory must exist. The default value is the current directory. |
| `envs` | map(string) | Environment variables for the session. |
| `hooks` | map(string) | A map of a tmux hook name to a command. |
| `options` | map(string) | A map of a tmux option name to a value. |
| `window` | block(s) | One or more windows. At least one window is required. |

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

The default `layout` is `tiled`. There are five presets: `even-horizontal`, `even-vertical`, `main-horizontal`, `main-vertical` and `tiled`. The attribute also accepts a **raw tmux layout string**, for example `"bb62,80x24,0,0"`. The `glaze save` command captures this string from a live window when no named preset applies. The `glaze up` command replays the string verbatim. Glazier validates the structure of the string at parse time. A malformed string fails fast. tmux recomputes the leading checksum. If you edit the geometry by hand and make an error, tmux rejects the layout when `up` runs. For a hand-authored profile, use a named preset. The raw string is exact but not easy to read.

### Pane

```hcl
pane {
  name     = "breach-protocol"
  focus    = true
  commands = ["nvim ./daemons", "echo upload ready"]

  size {                     # absolute resize
    x = "60%"                # cells ("80") or a percentage ("60%")
    y = "100"
  }

  adjust {                   # directional resize; a maximum of four blocks in order
    direction = "left"       # up | down | left | right
    amount    = "5"
  }

  options = { "remain-on-exit" = "on" }
}
```

Glazier sends the `commands` in order and serialises them with `tmux wait-for`. Each command completes before Glazier sends the next command. Glazier sends the **final** command without a wait. Thus a long-running or interactive command, for example `nvim` or a dev server, does not block the creation of the session. Glazier applies the `size` block first. The `adjust` blocks then refine the dimensions in order.

## Variables & string functions

### Declared variables (`var.`)

A profile declares its inputs with `variable` blocks in the Terraform style. You set a declared variable with `--var name=value`. You read it through the `var.` namespace and only that namespace. A `--var` flag with an undeclared name causes an error.

```hcl
variable "district" {
  description = "the district the gig is themed after"
  type        = string
  default     = "watson"
}

variable "fixer" {
  type = string
}

session {
  name = "gig-${var.district}"

  window {
    name = "${var.fixer}-ops"

    pane {
      commands = ["echo ${var.fixer} has the next job"]
    }
  }
}
```

```console
$ glaze up --var fixer=wakako                      # district gets its default value
$ glaze up --var district=arasaka --var fixer=wakako
```

A `variable` block takes three arguments:

| Argument | Required | Notes |
|----------|----------|-------|
| `type` | no | A bare keyword: `string`, `number` or `bool`. The default is `string`. Glazier converts the supplied value to this type. A value that cannot convert causes an error. |
| `default` | no | A literal value of the declared type. A variable **without** a default is required. |
| `description` | no | A literal string. It is documentation only. |

You supply values with the `--var name=value` flag or with a `--var-file <path>` flag. The `--var` flag is repeatable. A var file is a native HCL file of variable values. Glazier applies values in this order: the default first, then the var file, then each `--var` flag. The last value for a name applies.

Expressions can also reference `local.*` from `locals` blocks, `env.*` from `GLAZE_ENV_*` variables and `path.pwd` / `path.base`. Expressions can use inline `for` comprehensions.

> The `glaze down` command evaluates only the session `name`. A variable that
> only appears deeper in the profile is not required for teardown. Thus
> teardown stays idempotent.

### Built-in variables

Built-in namespaces sit alongside `var.`. They need no declaration:

- `env.*` exposes `GLAZE_ENV_*` environment variables without the prefix. Glazier reads `GLAZE_ENV_token=…` as `env.token`.
- `path.pwd` is the working directory. `path.base` is its basename.
- `local.*` reads the values that `locals` blocks declare.

```hcl
session {
  name               = "gig-${var.district}"
  starting_directory = path.pwd

  window {
    name = upper(path.base)

    pane {
      commands = ["deploy --token ${env.token}"]
    }
  }
}
```

```console
$ GLAZE_ENV_token=abc123 glaze up --var district=watson
```

### A working example: one profile for many gigs

Variables turn one `.glaze` file into a template for any project. This workspace starts an editor, a dev server and a log tail:

```hcl
variable "project" {
  description = "absolute path to the project you're jacking into"
  type        = string
}

variable "branch" {
  description = "branch shown in the session name"
  type        = string
  default     = "main"
}

variable "editor" {
  type    = string
  default = "nvim"
}

session {
  name               = "${var.branch}@${path.base}"
  starting_directory = var.project

  window {
    name  = "edit"
    focus = true

    pane {
      commands = ["${var.editor} ."]
    }
  }

  window {
    name = "run"

    pane {
      commands = ["npm run dev"]
    }

    pane {
      commands = ["tail -f ${var.project}/logs/dev.log"]
    }
  }
}
```

What each piece does:

- **`var.project` has no default**, thus it is required. If you omit it, glaze reports a missing variable and does not build a partial session.
- **`var.branch` and `var.editor` have defaults.** You ignore them in the common case. You override them only when necessary.
- The session **name** combines the branch and the basename of the current directory. Thus `feature-x@glazier` tells you what you see.
- **`starting_directory = var.project`** uses the value directly. The `${...}` wrapper is necessary only when you splice a value into a larger string.

The same file gives two different workspaces. The flags decide:

```console
$ glaze up --var project=$HOME/code/glazier                                  # nvim, on main
$ glaze up --var project=$HOME/code/glazier --var branch=feature-x --var editor=hx
```

### Glazier validates typed values

Each variable declares a `type`. Glazier validates and converts the supplied value before it starts a session:

```hcl
variable "base_index" {
  type    = number
  default = 1
}

variable "verbose" {
  type    = bool
  default = false
}

session {
  name = "typed-demo"

  options = {
    "base-index" = "${var.base_index}"
  }

  window {
    pane {
      commands = ["server --verbose=${var.verbose}"]
    }
  }
}
```

- `base_index` is a number. Glazier rejects `--var base_index=two` before the session starts. The message says that "two" is not a number.
- `verbose` is a boolean. `--var verbose=true` lands as `--verbose=true`. Glazier refuses each value that is not `true` or `false`.

You declare the inputs. Glazier makes sure that the profile sees only values of the correct type.

The functions are thin wrappers around the `go-cty` standard library, plus `random`:

- `chomp`
- `coalesce`
- `concat`
- `csvdecode`
- `format`
- `join`
- `jsondecode`
- `len`
- `lower`
- `random`
- `regexreplace`
- `replace`
- `reverse`
- `split`
- `strlen`
- `substr`
- `title`
- `trim`
- `trimprefix`
- `trimspace`
- `trimsuffix`
- `upper`

The `len` function counts the elements of a collection. The `strlen` function counts the characters of a string. The `random(list)` function returns a seeded random element of a list. It pairs naturally with a comprehension (`random([for e in local.editors : e])`).

See [SPEC.md](SPEC.md) for the full profile reference: blocks, variables, `locals`, built-in namespaces and the expression language.

## Development

```console
$ go test ./...              # run the test suite
$ go test -cover ./...       # run the test suite with coverage
$ go vet ./...               # run static analysis
$ go build ./...             # compile everything

$ make build                 # build bin/<os>-<arch>/glaze (version-stamped)
$ make test                  # run tests with gotestsum (pinned, run with `go run`)
$ make race                  # run tests under the race detector (CGO is required)
$ make cover                 # run tests with coverage; enforce the 80% floor
$ make vet                   # run go vet
$ make lint                  # run golangci-lint with gosec (pinned, run with `go run`)
$ make vuln                  # run the govulncheck vulnerability scan
$ make fuzz                  # find and run each native Go Fuzz* target
$ make all                   # deps > build > test > race > lint > cover > vuln
$ make install               # go install the version-stamped binary
$ make release               # cross-compile and zip (linux/darwin, amd64/arm64)
```

The `Makefile` pins each tool version and runs each tool with `go run <tool>@<version>`. No global installs or bootstrap scripts are necessary. The lint configuration is in [`.golangci.yml`](./.golangci.yml). Go **1.26+** is required. The `Makefile` and CI read the version from `go.mod`.

The test suite includes an end-to-end test, `pkg/tmux/e2e_test.go`. The test drives a real `tmux` server on a throwaway socket. The test skips itself when `tmux` is not on the `PATH`. The attacker-controllable surfaces, HCL profile decoding and variable collection, have native Go fuzz targets. The seed corpora replay as plain tests in each build. The `make fuzz` target runs real input generation.

CI runs on [Woodpecker](./.woodpecker/workflow.yaml) and [GitHub Actions](./.github/workflows/). Both call the same Makefile targets. A green local `make all` is a green build. GitHub also runs CodeQL, govulncheck, OpenSSF Scorecard and weekly scheduled fuzzing.

See [CONTRIBUTING.md](./CONTRIBUTING.md) for conventions and [SECURITY.md](./SECURITY.md) for the security policy and reporting channel.

## AI Disclosure

The architecture, the functionality and the base structure of this project are my own. I use AI as a tool for time-consuming work: documentation, tests and bug hunting. I also use it as a sounding board for structural decisions that keep the project easy to adopt and maintain. For a solo developer it is a force multiplier for high-quality code. There is _always_ a human as a final verification step. It is a tool for drudgery and toil, not a crutch.

## License

[MIT](LICENSE) © Wilhelm Murdoch
