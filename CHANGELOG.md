# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `glaze save` now captures **focus**: the active window and active pane are
  recorded as `focus = true`, so a saved profile restores which window/pane was
  in focus on the next `up`. Focus is inert and replay-safe (it only selects a
  pane), unlike commands/envs/hooks/options, which remain excluded.
- `glaze save` now captures **window layout**. tmux reports layout as a
  low-level coordinate string (e.g. `bb62,80x24,0,0`) rather than a named
  preset, so `save` records that raw string verbatim and `up` replays it
  exactly - restoring pane geometry even when it matches no named preset.
- The window `layout` attribute now accepts a **raw tmux layout coordinate
  string** in addition to the named presets (`even-horizontal`, `even-vertical`,
  `main-horizontal`, `main-vertical`, `tiled`). The value is structurally
  validated at parse time and fails fast when malformed.

## [0.1.0] - 2026-05-31

First public release. `glaze` can declaratively provision tmux workspaces from
an HCL `.glaze` profile, reformat and validate profiles, and capture a live
session back into a profile.

### Added
- `glaze up` - provision a tmux session, windows, and panes from a `.glaze` profile, attaching to it (or `--detached` to leave it in the background).
- `glaze format` - rewrite a profile to canonical HCL, with optional `--validate` to surface schema diagnostics.
- `glaze save` - capture a running session's structural layout (session, windows, panes, layout, and starting directories) back into a profile, to a file or `--stdout`.
- HCL-based profile syntax with Terraform-style diagnostics.
- Profile resolution from `--profile-path`, the current directory, or `$GLAZE_PATH`, with `~` expansion.
- Variable injection from `--var` flags and `GLAZE_ENV_*` environment variables, plus built-in `path.pwd` / `path.base` defaults.
- Template functions (`replace`, `upper`, `lower`, `join`, `trim`, and more) for use within profiles.
- Per session/window/pane environment variables, hooks, and tmux options.
- Per-pane `commands`, `size`, `adjust`, and `focus`; session-level `commands`.
- Reliable command sequencing via `tmux wait-for` instead of fixed sleeps.
- Base-index awareness (`base-index` / `pane-base-index`) read from tmux at runtime rather than assuming defaults.
- `--clear` to rebuild an existing session and `--detached` for headless use.

### Fixed
- Resolved several nil-pointer dereferences that could crash `glaze up` (failed session creation, invalid layout, malformed profiles, and malformed tmux output).
- Propagated HCL validation errors instead of printing them and continuing, which previously caused a segfault on invalid profiles.
- Guarded against creating a nested tmux session when running `glaze up` from within an existing session (now switches the client instead of attaching).
- Stopped re-provisioning a pre-existing session on `glaze up`, which duplicated windows; rebuilding is now opt-in via `--clear`.
- Corrected the swapped `--socket-path` / `--socket-name` arguments so each flag targets the intended tmux socket mechanism.
- Used `tmux list-sessions` rather than `server-info` to detect a running server, fixing a false negative when only detached sessions exist (e.g. in CI).

### Known limitations
- `glaze save` captures structural layout only (session/window/pane names, layout, and starting directories). It deliberately does **not** export pane commands, environment variables, hooks, or tmux options: commands and hooks would re-execute arbitrary code on the next `up`, environment variables can only be read as the whole session environment (leaking inherited secrets), and options read back as effective state mixed with `tmux.conf` and manual tweaks. Treat a saved profile as a layout scaffold to enrich by hand.

[Unreleased]: https://github.com/wilhelm-murdoch/glazier/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/wilhelm-murdoch/glazier/releases/tag/v0.1.0
