# Security Policy

## Supported versions

This project is pre-1.0. Security fixes are applied to the latest released `v0.x` minor; there are no long-term support branches yet.

| Version | Supported |
| ------- | --------- |
| latest `v0.x` | ✅ |
| older         | ❌ |

## Reporting a vulnerability

Please report security issues **privately** - do not open a public issue for anything exploitable.

- Preferred: [open a private security advisory](https://github.com/wilhelm-murdoch/glazier/security/advisories/new) ("Security" → "Report a vulnerability" on the repository).
- If that is unavailable, contact the maintainers privately through GitHub.

Please include enough detail to reproduce: affected version, a minimal proof of concept, and the impact you observed. We'll acknowledge the report, investigate, and coordinate a fix and disclosure timeline with you.

## Scope

What glaze touches that matters for security:

- **Profiles execute commands by design.** A `.glaze` profile's `commands`,
  `hooks`, and `options` are sent to your tmux server and run in your shell
  with your privileges. Applying an untrusted profile is equivalent to running
  an untrusted script - that is intended behavior, not a vulnerability. Reports
  along the lines of "a malicious profile can run commands" are out of scope;
  **a way to make a profile do something its text does not say** (command or
  argument injection through variable expansion, template functions, escaping
  bugs in the tmux command serialization) is very much in scope.
- **Untrusted input parsing.** The HCL parser, the variable/template expansion
  (`--var`, `GLAZE_ENV_*`), and the raw tmux layout-string validation all
  consume attacker-controllable text. Panics, hangs, or memory exhaustion on
  malformed profiles are in scope (these surfaces are fuzzed in CI).
- **Secret leakage.** `glaze save` deliberately refuses to export commands,
  hooks, environment variables, and options precisely so secrets in a live
  session can't end up in a file you might commit. Anything that defeats that
  boundary - `save` output containing session environment values, for example -
  is a valuable report.
- **No network exposure.** glaze talks only to a local tmux socket; it makes no
  network connections of its own.
