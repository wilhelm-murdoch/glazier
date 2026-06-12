---
name: Bug report
about: Something isn't working as expected
title: ""
labels: bug
---

### Summary

A clear, concise description of the bug.

### Version

- `glaze --version` output:
- tmux version (`tmux -V`):
- OS (Linux distro or macOS version):

### Reproduction

The smallest `.glaze` profile and command that trigger it:

```hcl
session {
  # ...
}
```

```console
$ glaze up ...
```

### Expected behavior

What you expected to happen.

### Actual behavior

What actually happened. Include the full error or diagnostic output if there
is any (`--log-level debug` and `glaze up --debug` help here).

### Additional context

Anything else that helps - custom socket, tmux.conf settings in play, etc.
