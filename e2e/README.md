# End-to-end tests

This module tests the compiled `glaze` binary against real tmux servers. Each case runs `glaze` as a user does and then reads the result back from tmux with plain tmux commands. The module never imports glaze, so a bug in glaze cannot hide itself in a check.

The module is separate from glaze (`e2e/go.mod`). The compiler thus stops a case from importing a package of glaze, and `go test ./...` in the root does not run these cases.

## Layout

| Path                      | Content                                                                 |
| ------------------------- | ----------------------------------------------------------------------- |
| `harness/`                | The library: cases, processes, checks, tmux queries, golden files.      |
| `cases/`                  | The cases. One file for each area of glaze.                             |
| `fixtures/<area>/*.glaze` | The profiles that the cases use. Each one is a real, formatted profile. |
| `golden/`                 | The expected output of commands such as `save`, `format` and errors.    |
| `expected-failures.txt`   | The baseline: the checks that are known to fail, with a reason.         |
| `images/Dockerfile`       | One image for each tmux target.                                         |
| `cmd/matrix/`             | The runner: it builds the images and runs the cases on each target.     |

## Requirements

- Go, at the version in `go.mod`.
- tmux, for a run on the host.
- Docker, for a run on the targets. OrbStack is sufficient on macOS.

## Run the cases on the host

Use this for fast feedback while you write a case. The host tmux is one target only.

```bash
make e2e
```

`make e2e` builds glaze for the host and runs every case against the tmux in `PATH`. To run some cases only, set `RUN` to a `go test -run` pattern:

```bash
make e2e RUN='TestCommands/cmd_serial'
```

Each case has its own tmux server in its own `TMUX_TMPDIR`. A case never touches the tmux server that you use.

To run the cases without make, give the path of the binary in `GLAZE_BIN`:

```bash
GLAZE_BIN=$PWD/bin/darwin-arm64/glaze go test -count=1 ./cases/
```

Without `GLAZE_BIN`, every case skips. Set `E2E_REQUIRE=1` to make a missing binary a failure.

## Run the cases on all targets

```bash
make e2e-matrix
```

`make e2e-matrix` builds one image for each target, cross-compiles glaze and the test binary for Linux and runs all targets in parallel. Each target writes a JSON lines report to `e2e/results/<target>.jsonl`. The runner then prints a table and a list of the checks that failed.

To compare the working tree with a base revision, set `BASE`:

```bash
make e2e-matrix BASE=github/main
```

The runner builds glaze from the base revision and runs the cases of the working tree on both binaries. It prints one table that you can paste into a pull request, then the checks that the change fixes and the checks that it breaks.

### Targets

| Target     | Base image             | tmux               |
| ---------- | ---------------------- | ------------------ |
| `bookworm` | `debian:bookworm-slim` | 3.3a               |
| `trixie`   | `debian:trixie-slim`   | 3.5a               |
| `jammy`    | `ubuntu:22.04`         | 3.2a               |
| `noble`    | `ubuntu:24.04`         | 3.4                |
| `alpine`   | `alpine:3.22` (musl)   | 3.5a               |
| `tmux37`   | `debian:bookworm-slim` | 3.7c, from source  |

Each image also has bash, zsh, fish and dash, because the command delivery of glaze depends on the shell. Each base image is pinned by digest, and Dependabot updates the digests.

The `tmuxnext` target builds tmux from its development branch. It is not in the default set, because a change in tmux must not block glaze. The weekly canary runs it:

```bash
make e2e-matrix TARGETS=tmuxnext
```

The runner resolves the branch to a commit, so a new commit of tmux builds a new image. Use `MATRIX_FLAGS='-tmux-ref <branch or tag>'` to build another reference.

The containers run the cases as your own user, not as root. A permission check, for example a read-only profile, then behaves as it does for a user.

### Runner options

Give runner options in `MATRIX_FLAGS`, or run `go run ./cmd/matrix -h` in `e2e/` for the full list.

| Option                  | Meaning                                                                 |
| ----------------------- | ----------------------------------------------------------------------- |
| `-targets a,b`          | Run these targets only.                                                 |
| `-run PATTERN`          | Run the cases that match a `go test -run` pattern.                      |
| `-base REV`             | Also test glaze from a git revision, and compare.                       |
| `-glaze FILE`           | Test this Linux binary, for example a release, instead of the working tree. |
| `-expect-version V`     | Check that `glaze --version` reports `V`.                               |
| `-parallel N`           | Run `N` cases at the same time on each target.                          |
| `-markdown FILE`        | Also write the summary to `FILE`.                                       |
| `-count N`              | Run each case `N` times, to find a flaky check.                          |
| `-cpus N`               | Limit each container to `N` CPUs, as on a small CI runner.              |
| `-arch amd64\|arm64`    | Build and run for this architecture. The default is that of Docker.    |

## Write a case

A case is a function in a file of `cases/`. `harness.Run` gives it a new `harness.Case` and runs it in parallel with the other cases.

```go
func TestDirectories(t *testing.T) {
	harness.Run(t, "dirs_relative", func(c *harness.Case) {
		c.Mkdir("prof/sub", "elsewhere")
		c.Fixture("directories/relative.glaze", "prof/.glaze")
		c.Cd("elsewhere")
		c.OK(c.Up("--profile-path", "../prof/.glaze"), "up with a relative starting_directory from another directory")
		c.EventuallyPanePaths("relative starting_directory is relative to the profile", c.Path("prof/sub"), "=relative:")
	})
}
```

### The case

| Item                   | Meaning                                                                |
| ---------------------- | ---------------------------------------------------------------------- |
| `c.Dir`                | The work directory. Each file helper is relative to it.                |
| `c.Home`               | `HOME` of every process. It is `c.Dir/home`.                           |
| `c.Socket`             | The `-L` name of the tmux server of the case.                          |
| `c.Path(rel)`          | The absolute path of `rel` in the work directory.                      |
| `c.Cd(rel)`            | Changes the directory in which the next commands run.                  |
| `c.Fixture(name, dst)` | Copies `fixtures/<name>` to `dst`. The default `dst` is `.glaze`.      |
| `c.Simple(name, dst)`  | Writes a profile with one session, one window `w` and one pane `p`.    |
| `c.Write(rel, text)`   | Writes a file and its parent directories.                              |
| `c.TmuxConf(text)`     | Writes `~/.tmux.conf`, which the server of the case reads at start.    |
| `c.ShortDir()`         | Makes a directory with a short path, for a `--socket-path` socket.     |
| `c.ShortSocket()`      | Returns a socket path in a new `ShortDir`.                             |

### Commands

| Helper                     | Runs                                                             |
| -------------------------- | ---------------------------------------------------------------- |
| `c.Up(args...)`            | `glaze up --detached --socket-name <socket> args...`             |
| `c.StartUp(opts, args...)` | `c.Up` in the background, for a case that signals glaze.         |
| `c.Down(args...)`          | `glaze down --socket-name <socket> args...`                      |
| `c.Save(args...)`          | `glaze save --socket-name <socket> args...`                      |
| `c.Ls(args...)`            | `glaze ls --socket-name <socket> args...`                        |
| `c.Glaze(args...)`         | `glaze args...`                                                  |
| `c.Exec(opts, name, ...)`  | Any program, with the environment of the case.                   |
| `c.Start(opts, name, ...)` | A program in the background. `Wait` and `Signal` control it.     |
| `c.StartTerminal(...)`     | A program on a pseudo-terminal of 120 columns and 40 rows.       |
| `c.AttachControl(session)` | A tmux control client, attached until `Close`.                   |
| `c.Type(target, line)`     | Types a line into a pane and presses Enter, as a user does.      |
| `c.KillServer()`           | Stops the server of the case and waits until it is gone.         |

Use `harness.ShellQuote` for each word of a line that you type into a pane. `r.Describe()` summarises a result for the detail of a `c.True` check. `r.Succeeded()` is an exit 0 before the deadline and `r.Failed()` is a non-zero exit before the deadline.

A command that only prepares a case, for example an `up` before a `save`, is not a check. Wrap it in `c.Must(r, "step")`: when it fails, the case stops with the reason, and a later check does not fail for a misleading reason. Use `c.TmuxSetup(args...)` for a tmux command that prepares a case, for example a session that glaze must leave alone, and `c.Tmux(args...)` only for a query.

Each command runs in a process group of its own. After its deadline (20 s, or `Opts.Timeout`), it gets SIGTERM, and 2 s later SIGKILL. A `Result` with `TimedOut` set is a hang. `Fails` does not accept a hang as a failure.

Every process starts with an empty environment plus `PATH`, `HOME`, `TERM`, `SHELL`, `TMUX_TMPDIR` and the locale. A `TMUX` or `GLAZE_*` variable of the host never reaches a case. Use `Opts.Env` to add a variable.

### Checks

Each check takes a label first and records one line in the report.

| Check                                 | Passes when                                  |
| ------------------------------------- | -------------------------------------------- |
| `c.OK(r, label)`                      | `r` exits 0. The label gets ` rc=0`.         |
| `c.Fails(r, label)`                   | `r` exits non-zero before its deadline. The label gets ` rc!=0`. |
| `c.ExitCode(r, label, n)`             | `r` exits `n`.                               |
| `c.Finishes(r, label)`                | `r` ends before its deadline.                |
| `c.Within(r, label, d)`               | `r` ends in less than `d`.                   |
| `c.AtLeast(r, label, d)`              | `r` takes `d` or longer.                     |
| `c.Equal(label, want, got)`           | `got` is `want`.                             |
| `c.Match(label, re, s)`               | `s` matches `re`.                            |
| `c.NoMatch(label, re, s)`             | `s` does not match `re`.                     |
| `c.True(label, cond, detail, ...)`    | `cond` is true.                              |
| `c.SessionExists(label, name)`        | The server has the session.                  |
| `c.SessionGone(label, name)`          | The server does not have the session.        |
| `c.NoServer(label)`                   | No server runs on the socket of the case.    |
| `c.EventuallyEqual(label, want, fn)`  | `fn()` returns `want` within 5 s.            |
| `c.EventuallyMatch(label, re, fn)`    | `fn()` matches `re` within 5 s.              |
| `c.EventuallyExists(label, rel)`      | The file `rel` appears within 5 s.           |
| `c.EventuallyPanePaths(label, want, target)` | The pane paths of `target` become `want` within 5 s. |
| `c.Eventually(label, d, fn)`          | `fn()` returns true within `d`.              |
| `c.Golden(label, name, got)`          | `got` is the content of `golden/<name>`.     |

Obey these rules for a label:

- Make each label unique in its case. The harness stops a case that uses a label twice.
- Keep each label the same on every run and every target. Do not put a path, a time, a process id or a tmux version in a label. The harness stops a case with a path of the run in a label.
- Write what the check expects, for example `window order` or `--clear inside the target says why`.
- Build a label with a value with `fmt.Sprintf`, for example `fmt.Sprintf("invalid layout [%s] rejected", layout)`.

Match a diagnostic or a log line on `r.Stderr`, where glaze writes them. Use `r.Output()` only when the stream does not matter.

A check that fails does not stop the case. Use `c.T().Fatal` only when the next steps cannot run.

### Wait for a result, do not sleep

Do not use `time.Sleep` in a case. tmux and the shells in panes work in the background, and a fixed sleep is too long on a fast host and too short on a slow one. Use `EventuallyEqual`, `EventuallyMatch` or `Eventually` for a check, and `WaitUntil` or `WaitFile` to wait before the next step.

Some state is ready only some time after glaze exits, so read it with an `Eventually` check:

- the path of a pane: use `EventuallyPanePaths`. tmux reads the path from the process in the pane, which changes to its directory after tmux starts it.
- a file that a command in a pane writes.
- a process, for example a tmux client that glaze stops: it exits after glaze.

A name, an option, a hook, a layout and the order of windows and panes are ready when glaze exits, so a plain check is correct for them.

A step that reads the paths of panes as a reference, or runs `save`, cannot wait for a known value. Call `c.WaitForPanes(session)` first: it waits until each pane runs its own program, so each pane path is final.

To find a flaky check before CI does, run the cases many times on small containers:

```bash
make e2e-matrix MATRIX_FLAGS='-count 5 -cpus 1'
```

### Read tmux

The tmux helpers read the state with plain tmux commands: `Sessions`, `WindowNames`, `WindowIndexes`, `WindowCount`, `WindowID`, `ActiveWindow`, `PaneTitles`, `PaneIndexes`, `PaneCount`, `PanePaths`, `ActivePane`, `PaneSize`, `Geometry`, `ReferenceGeometry`, `Option`, `ClientSessions` and `State`. Use `c.Tmux(args...)` for any other query. `c.TmuxOn(socket, ...)` reaches a second server of the case, `c.TmuxAt(path, ...)` reaches a server on a socket path and `c.TmuxDefault(...)` reaches the default server in the `TMUX_TMPDIR` of the case.

`c.State(session, harness.WindowState, harness.PaneState)` returns every window and pane of a session on one line each. Compare two states to show that a step left a session as it was. When a check fails, `c.Snapshot(session)` writes the state to the log.

Find a window by its id (`c.WindowID`) or with an exact target such as `=name:`. Do not use an index: a `tmux.conf` can change the base index.

To start a new server in the same case, use `c.KillServer()`, not `c.Tmux("kill-server")`. It waits until the process of the old server is gone. A stopping server can stop answering before its process exits, and a command that connects in that moment fails with "server exited unexpectedly" or gives its session to the old server.

### Observations

Use `c.Logf` for an observation that has no expected result, for example a duration. A log line is not a check, and the report does not show it. When glaze has a defined behavior, write a check for it.

## Fixtures

A fixture is a profile in `fixtures/<area>/`. Rules:

- Write each fixture in the canonical format. `TestFixtures` fails when `glaze format` changes a fixture. Run `make e2e-fmt` to format all fixtures.
- Give each fixture a name that says what it contains, for example `directories/levels.glaze`.
- Use a relative `starting_directory` or `~`. glaze resolves a relative path against the directory of the profile, so a fixture needs no absolute path.
- Use a variable for a value that a case changes, for example `layout = var.layout`, and give the value with `--var`.
- Put a comment at the top when the purpose is not clear from the content.
- Reuse a fixture when two cases need the same profile.

A case can also write a profile with `c.Write`, for example a profile with a hostile name. Use `harness.Quote` and `harness.List` to write HCL strings, in a raw-string template.

`fixtures/malformed/` holds every fixture that is not in the canonical format: the profiles that glaze must reject and the input of the `format` cases. `TestFixtures` skips this directory. The label of a malformed case names its file, so a comment in the file does not change the check.

## Style

The module follows the style of glaze, with these rules for cases:

- Give each `Test` function a doc comment of one sentence, and each case a purpose comment above `harness.Run` when its name does not say it.
- Start the names of all cases of one `Test` with the same prefix, for example `cmd_` in `TestCommands`.
- Give each session in a fixture a name that says what it is, and use a different name in each fixture of an area.
- Name a magic value with a constant: the exit codes and `hangTimeout` are in `cases/helpers_test.go`.
- Put a blank line after a statement that ends with a closing brace, for example an `if` or a `for`, unless the enclosing block ends there too. A `case` label and `} else {` need no blank line. `make e2e-check` checks this rule for all Go code in the repository.

## Golden files

`c.Golden(label, name, got)` compares output with `golden/<name>`. It replaces the work directory with `$WORK`, the home directory with `$HOME` and removes the time from each log line. To write the golden files again after a change, run the case with `-update`:

```bash
GLAZE_BIN=... go test ./cases/ -run TestFormat -update
```

Read the diff of the golden files before you commit it.

## The baseline

`expected-failures.txt` names the checks that are known to fail. Each line has four fields, separated with ` | `:

```text
TARGET | TEST/CASE | CHECK | REASON
*      | TestCommands/cmd_exit | non-final 'exit' command does not hang | glaze waits forever; see the issue
```

`TARGET` is a target name or `*` for all targets. A check in the baseline that fails is an expected failure. A check in the baseline that passes fails the run, so remove the entry when you fix the bug.

## Reports

With `E2E_REPORT` set, the harness appends one JSON object for each check to that file:

```json
{"target":"bookworm","test":"TestLayouts/layout_tiled_3","check":"up tiled x3 rc=0","status":"pass"}
```

`status` is `pass`, `fail`, `xfail` (an expected failure) or `xpass` (an unexpected pass). The runner reads these reports to compare targets and revisions.

## Continuous integration and releases

| Where                         | What runs                                                                 | Blocks |
| ----------------------------- | ------------------------------------------------------------------------- | ------ |
| `make all`, Woodpecker, CI    | `make e2e-check`: vet, lint, unit tests and govulncheck of this module.   | Yes    |
| Woodpecker                    | `make e2e` on the tmux of the Go image, as a user that is not root.       | Yes    |
| CI, linux/amd64               | The matrix on all six targets. A pull request compares with its base.     | Yes    |
| CI, linux/arm64 and macOS     | The matrix on arm64, and `make e2e` on the macOS host.                    | Yes    |
| Release, linux/amd64          | The matrix on the release zip, with `-expect-version` set to the tag.     | Yes    |
| Release, linux/arm64 and macOS| The same on the arm64 zip, and on the darwin zip on the macOS host.       | Yes    |
| E2E canary, each Monday       | The matrix on `tmuxnext`.                                                 | No     |

Run the release workflow by hand for a dry run before you push a tag. It builds the zips and runs every e2e job on them, but it does not publish.

The release publishes only when the cases pass on each zip that it ships: linux/amd64 and linux/arm64 on all six targets, and darwin/arm64 on the macOS host. The darwin/amd64 zip has no runner of its own; it shares its source and its tests with darwin/arm64.
