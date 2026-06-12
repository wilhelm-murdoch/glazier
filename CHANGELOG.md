# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-06-12

First public release. `glaze` can declaratively provision tmux workspaces from
an HCL `.glaze` profile, tear them down and list them, reformat and validate
profiles, and capture a live session back into a profile.

### Added
- `glaze up` - provision a tmux session, windows, and panes from a `.glaze` profile, attaching to it (or `--detached` to leave it in the background).
- `glaze down` - tear down the session described by the resolved profile, or `--session <name>` directly with no profile required. Interpolated session names resolve through the same `--var`/`GLAZE_ENV_*` machinery as `up`, and bringing down a session that is not running is a no-op rather than an error, so `down` stays idempotent for scripts.
- `glaze ls` - list the sessions running on the target tmux server with their window counts and starting directories; the session the current client is attached to is marked with an asterisk.
- `glaze format` - rewrite a profile to canonical HCL, with optional `--validate` to surface schema diagnostics.
- `glaze save` - capture a running session's structural layout (session, windows, panes, layout, focus, and starting directories) back into a profile, to a file or `--stdout`.
- HCL-based profile syntax with Terraform-style diagnostics.
- Profile resolution from `--profile-path`, the current directory, or `$GLAZE_PATH`, with `~` expansion.
- Variable injection from `--var` flags and `GLAZE_ENV_*` environment variables, plus built-in `path.pwd` / `path.base` defaults.
- Template functions (`replace`, `upper`, `lower`, `join`, `trim`, and more) for use within profiles.
- Per session/window/pane environment variables, hooks, and tmux options.
- Per-pane `commands`, `size`, `adjust`, and `focus`; session-level `commands`.
- The window `layout` attribute accepts both the named presets (`even-horizontal`, `even-vertical`, `main-horizontal`, `main-vertical`, `tiled`) and a **raw tmux layout coordinate string** (e.g. `bb62,80x24,0,0`) - the form `save` captures verbatim from a live window and `up` replays exactly, restoring pane geometry even when it matches no named preset. Raw strings are structurally validated at parse time and fail fast when malformed.
- Reliable command sequencing via `tmux wait-for` instead of fixed sleeps. The final command in each list is sent fire-and-forget so a long-running or interactive command (`nvim`, `tail -f`, a dev server) never blocks session creation.
- Base-index awareness (`base-index` / `pane-base-index`) read from tmux at runtime rather than assuming defaults.
- `--clear` to rebuild an existing session and `--detached` for headless use.
- Native Go **fuzz targets** for the attacker-controllable surfaces: the full HCL parse/decode pipeline (including the layout-string and size validators) and `--var`/`GLAZE_ENV_*` variable collection. The seed corpora run as plain tests in every build; `make fuzz` (and weekly CI crons) run real input generation.
- Release binaries for linux/darwin on amd64/arm64, shipped with integrity material: a `SHA256SUMS` file (via `make checksums`) and a signed sigstore build-provenance attestation, verifiable with `gh attestation verify <zip> --repo wilhelm-murdoch/glazier`. The release workflow re-verifies the tagged commit (vet, tests, race) before publishing. There is no Windows build - tmux has no native Windows build, so a Windows `glaze` binary could never work.
- Repository hygiene from the shared Go template: SHA-pinned GitHub workflows (CI matrix, CodeQL, govulncheck, OpenSSF Scorecard, scheduled fuzzing, release), grouped weekly Dependabot updates, issue/PR templates, `SECURITY.md`, `CONTRIBUTING.md`, and `make race`/`cover` (with an 80% floor)/`vuln`/`fuzz` targets mirrored across the Woodpecker pipeline, with `gosec` running as part of `make lint` in both CIs.

### Fixed
- Resolved several nil-pointer dereferences that could crash `glaze up` (failed session creation, invalid layout, malformed profiles, and malformed tmux output).
- Propagated HCL validation errors instead of printing them and continuing, which previously caused a segfault on invalid profiles.
- Guarded against creating a nested tmux session when running `glaze up` from within an existing session (now switches the client instead of attaching).
- Stopped re-provisioning a pre-existing session on `glaze up`, which duplicated windows; rebuilding is now opt-in via `--clear`.
- Corrected the swapped `--socket-path` / `--socket-name` arguments so each flag targets the intended tmux socket mechanism.
- Used `tmux list-sessions` rather than `server-info` to detect a running server, fixing a false negative when only detached sessions exist (e.g. in CI).
- `glaze up` no longer hangs forever when a pane or session `commands` entry is long-running or interactive; earlier commands in a list are still serialised via `tmux wait-for`.

### Known limitations
- `glaze save` captures structural layout only (session/window/pane names, layout, focus, and starting directories). It deliberately does **not** export pane commands, environment variables, hooks, or tmux options: commands and hooks would re-execute arbitrary code on the next `up`, environment variables can only be read as the whole session environment (leaking inherited secrets), and options read back as effective state mixed with `tmux.conf` and manual tweaks. Treat a saved profile as a layout scaffold to enrich by hand.

[Unreleased]: https://github.com/wilhelm-murdoch/glazier/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/wilhelm-murdoch/glazier/releases/tag/v0.1.0
