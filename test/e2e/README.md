# Release binary E2E harness

This harness tests the compiled `glaze` binary from a GitHub release. It does not test the source. Each test starts a real tmux server on a private socket. The test then reads the result back from tmux with `list-sessions`, `list-windows`, `list-panes`, `show-options` and `show-hooks`.

The harness is interim. It lives on the `test/bootstrap-harness` branch until it moves into CI.

## Requirements

- Docker. OrbStack is sufficient on macOS.
- Network access to GitHub releases.

## Targets

| Target     | Base image             | tmux |
| ---------- | ---------------------- | ---- |
| `bookworm` | `debian:bookworm-slim` | 3.3a |
| `trixie`   | `debian:trixie-slim`   | 3.5a |
| `jammy`    | `ubuntu:22.04`         | 3.2a |
| `noble`    | `ubuntu:24.04`         | 3.4  |
| `alpine`   | `alpine:3.22` (musl)   | 3.5a |

The default run uses `bookworm`, `trixie`, `jammy` and `alpine`.

Each base image is pinned by digest. The `Dockerfile` has one stage for each Debian and Ubuntu target, named after the target, and `matrix.sh` selects it with `--build-arg BASE=<target>`. To add a target, add a pinned stage with that name. Dependabot updates the digests every week. It does not change a tag, because each tag is a test target.

## Run the harness

1. Start Docker.
2. Run `./matrix.sh` from this directory.

```bash
./matrix.sh v0.1.6
```

To test a different release, give its tag as the first argument. To test specific targets, give their names after the tag.

```bash
./matrix.sh v0.1.6 bookworm jammy
```

To run only some test groups, set `ONLY` to a regular expression.

```bash
ONLY='t_commands|t_save' ./matrix.sh v0.1.6 bookworm
```

Each image downloads `glaze-linux-<arch>.zip` and `SHA256SUMS` from the release. The build stops if the checksum does not match.

`matrix.sh` passes the release version to the harness as `EXPECT_VERSION`, and `cli_basics` checks that `glaze --version` reports it. Without `EXPECT_VERSION`, for example for a local build, `cli_basics` checks only the form of the version line.

## Results

The harness writes its output to `results/`. Git ignores this directory.

| Path                            | Content                                            |
| ------------------------------- | -------------------------------------------------- |
| `results/<target>.out`          | The console output of the run.                     |
| `results/<target>/results.tsv`  | One row for each assertion: test, assertion, status, detail. |
| `results/<target>/logs/`        | One log for each test, with each command, exit code, stdout and stderr. |
| `results/<target>/env.txt`      | The tmux version, the glaze version and the kernel. |
| `results/<target>.fail`         | The sorted list of failed assertions.              |

An assertion has one of three statuses:

- `PASS`: the binary did what the README or SPEC says.
- `FAIL`: the binary did not do what the README or SPEC says.
- `INFO`: an observation with no fixed expected result. Read these rows when you examine an edge case.

`matrix.sh` compares the failure lists of all targets. A difference between two targets shows a tmux version dependency. `matrix.sh` returns exit code 1 when a target has a failure.

## Files

| File                | Content                                                         |
| ------------------- | --------------------------------------------------------------- |
| `matrix.sh`         | Builds one image for each target and runs the targets in parallel. |
| `run.sh`            | Runs all test groups inside one container.                      |
| `lib.sh`            | Assertion helpers, tmux introspection helpers and fixture helpers. |
| `t10_core.sh`       | CLI, profile resolution, structure, base index, directories, layouts and focus. |
| `t20_panes.sh`      | Size, adjust, commands, options, hooks and envs.                |
| `t30_vars.sh`       | Variables, locals, functions, idempotence, `down`, `ls` and `format`. |
| `t40_save.sh`       | `save`, hostile names, sockets, attach, scale and malformed profiles. |
| `t50_extra.sh`      | Target ambiguity, diagnostics and recovery after a failed `up`. |
| `Dockerfile`        | The Debian and Ubuntu image.                                    |
| `Dockerfile.alpine` | The Alpine image.                                               |

## Add a test

1. Write a function with a `t_` prefix in a `t*.sh` file.
2. Start each case with `begin <name>`. This gives the case a new socket, a new working directory and a new `HOME`.
3. Write the profile with `fx`. `fx` replaces `@WD@` with the working directory and `@HOME@` with the home directory.
4. Run the binary with `up`, `down` or `gz`. These helpers record `RC`, `OUT`, `ERR` and `DUR`.
5. Check the result with `eq`, `match`, `nomatch`, `rc0`, `rcnz`, `exists`, `gone` or `no_server`.
6. End each case with `end`. This stops the tmux server of the case.
7. Add the function name to `GROUPS_ALL` in `run.sh`.

A `~/.tmux.conf` in `$HOME` applies to the server of the case. Use it to test options such as `base-index`.

Put a timeout on a command that can hang. Set `TO` before the helper, for example `TO=10 up`. A timeout gives exit code 124.

## Known results for v0.1.6

The sweep for v0.1.6 gives `PASS=433 FAIL=88 INFO=49` on all four default targets. The failure lists are identical. Thus these failures are glaze bugs and not tmux version dependencies.

The `noble` target (tmux 3.4) gives `PASS=431 FAIL=90 INFO=49`. The two extra failures are in `save_escaping`. tmux 3.4 writes `$` as `\$` in `-F` output, and glaze does not remove the escape.

Each fix branch must remove its failures from this list and must not add new failures.
