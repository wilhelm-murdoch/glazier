# Glazier profile specification

This document is the reference for the `.glaze` profile format. It describes the blocks, the attributes and the expression language. The [README](README.md) gives a walkthrough of the same material.

A profile is a native HCL file. A profile contains exactly one `session` block. A profile can also contain `variable` blocks and `locals` blocks at the top level. Only a `variable` block has a label. The label is the name of the variable. You name a `session`, a `window` or a `pane` with its `name` attribute.

---

## 1. Blocks

### 1.1 `session`

The `session` block is the single root block. It describes the tmux session.

```hcl
session {
  name               = "daemon-run"     # the default value is "default"
  starting_directory = "~/runs/arasaka" # the default value is the current directory
  commands           = ["echo jacked-in"]

  envs = {
    EDITOR     = "nvim"
    ICE_TARGET = "arasaka-mainframe"
  }

  hooks   = { "session-created" = "run-shell 'echo jacked-in'" }
  options = { "base-index" = "1" }

  window {
    # ...at least one window is required
  }
}
```

| Attribute            | Type         | Notes                                                          |
| -------------------- | ------------ | -------------------------------------------------------------- |
| `name`               | string       | The session name. The default value is `default`.              |
| `starting_directory` | string       | The directory must exist. The default value is the current directory. |
| `commands`           | list(string) | Commands for the active pane of the session. See 1.4.          |
| `envs`               | map(string)  | Environment variables for the session.                         |
| `hooks`              | map(string)  | A map of a tmux hook name to a command.                        |
| `options`            | map(string)  | A map of a tmux option name to a value.                        |
| `window`             | block(s)     | One or more windows. At least one window is required.          |

Glazier runs the session `commands` in the active pane of the session. Glazier runs them after it creates all windows and panes. The rules in 1.4 apply to these commands.

### 1.2 `window`

```hcl
window {
  name   = "ice-breaker"
  layout = "main-vertical"   # see below
  focus  = true              # make this the active window

  hooks   = { "window-renamed" = "display 'trace detected'" }
  options = { "automatic-rename" = "off" }

  pane {
    # ...at least one pane is required
  }
}
```

| Attribute            | Type        | Notes                                                          |
| -------------------- | ----------- | -------------------------------------------------------------- |
| `name`               | string      | The window name. The default value is `default`.               |
| `starting_directory` | string      | The directory must exist. The default value is the current directory. |
| `layout`             | string      | A named preset or a raw tmux layout string. See below.         |
| `focus`              | bool        | Make this the active window.                                   |
| `hooks`              | map(string) | A map of a tmux hook name to a command.                        |
| `options`            | map(string) | A map of a tmux option name to a value.                        |
| `pane`               | block(s)    | One or more panes. At least one pane is required.              |

The default `layout` is `tiled`. There are five presets: `even-horizontal`, `even-vertical`, `main-horizontal`, `main-vertical` and `tiled`. The attribute also accepts a raw tmux layout string, for example `"bb62,80x24,0,0"`. The `glaze save` command captures this string from a live window. The `glaze up` command replays the string without change. Glazier validates the structure of the string at parse time. A malformed string causes an error.

### 1.3 `pane`

```hcl
pane {
  name     = "breach-protocol"
  focus    = true
  commands = ["nvim ./daemons", "echo upload ready"]

  size {                     # absolute resize
    x = "60%"                # cells ("80") or a percentage ("60%")
    y = "100"
  }

  adjust {                   # directional resize
    direction = "left"       # up | down | left | right
    amount    = "5"
  }

  options = { "remain-on-exit" = "on" }
}
```

| Attribute            | Type         | Notes                                                          |
| -------------------- | ------------ | -------------------------------------------------------------- |
| `name`               | string       | The pane title. The default value is `default`.                |
| `starting_directory` | string       | The directory must exist. The default value is the current directory. |
| `focus`              | bool         | Make this the active pane.                                     |
| `commands`           | list(string) | Commands for the pane, in order. See 1.4.                      |
| `hooks`              | map(string)  | A map of a tmux hook name to a command.                        |
| `options`            | map(string)  | A map of a tmux option name to a value.                        |
| `size`               | block        | An absolute resize. See below.                                 |
| `adjust`             | block(s)     | A directional resize. A maximum of four blocks. See below.     |

A `size` block must contain `x`, `y` or both. A value is a count of cells, for example `"80"`. A value can also be a percentage, for example `"60%"`. A count must be a positive integer.

An `adjust` block must contain `direction` and `amount`. The `direction` value is `up`, `down`, `left` or `right`. The `amount` value has the same format as a `size` value. Glazier applies the `size` block first. Glazier then applies each `adjust` block in declaration order.

### 1.4 Command execution

Glazier serialises the commands in a list with `tmux wait-for`. Each command completes before Glazier sends the next command. Glazier does not wait for the final command in a list. Thus a long-running or interactive final command does not block the creation of the session.

---

## 2. Variables

A profile declares its inputs with `variable` blocks. You read a declared variable through the `var.` namespace and only that namespace. A supplied value for a name that no block declares causes an error.

```hcl
variable "district" {
  description = "the district the gig is themed after"
  type        = string       # optional; the default is string
  default     = "watson"     # no default makes the variable required
}
```

| Attribute     | Required | Notes                                                                 |
| ------------- | -------- | --------------------------------------------------------------------- |
| `type`        | no       | A bare keyword: `string`, `number` or `bool`. The default is `string`. |
| `default`     | no       | A literal value. Glazier converts it to the declared type.            |
| `description` | no       | A literal string. It is documentation only.                           |

Glazier converts each supplied value and each default to the declared type. A value that cannot convert causes an error. For example `--var count=two` for a `number` causes an error before Glazier starts a session.

You supply values with the `--var name=value` flag or with a `--var-file <path>` flag. The `--var` flag is repeatable. A var file is a native HCL file of `name = value` attributes. JSON var files are not supported.

Glazier applies values in this order: the default first, then the var file, then each `--var` flag. The last value for a name wins.

An unset required variable causes an error. A `--var` flag or a var file entry with an undeclared name causes an error.

> The `glaze down` command evaluates only the session `name`. A variable that only appears deeper in the profile is not required for teardown. Thus teardown stays idempotent.

---

## 3. `locals`

A `locals` block declares named values for use across the profile, for example derived strings or computed lists. A local can reference `var.*`, `env.*`, `path.*`, the function library and other locals. The declaration order of locals has no effect. A circular reference causes an error. A duplicate local name causes an error. You read a local as `local.<name>`.

```hcl
locals {
  slug     = lower(replace(var.district, " ", "-"))
  session  = "gig-${local.slug}"
  editors  = ["nvim", "hx", "vim"]
}
```

The CLI cannot set a local. A local exists so that you declare a value once and reuse it.

---

## 4. Expressions

Each attribute value is an HCL expression. An expression has access to this context:

- `var.<name>` is a declared variable.
- `local.<name>` is a resolved local.
- `env.<name>` is an environment variable with the `GLAZE_ENV_` prefix. Glazier removes the prefix. For example `GLAZE_ENV_token` becomes `env.token`.
- `path.pwd` is the current directory. `path.base` is the basename of the current directory.
- String interpolation and templates: `name = "gig-${var.district}"`.
- Inline comprehensions with native HCL `for` expressions: `[for e in local.editors : upper(e)]`.

### 4.1 Function library

The functions are thin wrappers around the [go-cty](https://github.com/zclconf/go-cty) standard library, plus `random`:

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

The `len` function counts the elements of a collection. The `strlen` function counts the characters of a string. The `random(list)` function returns one random element of a list as a string. The result changes between runs. An empty list causes an error. The function pairs naturally with a comprehension:

```hcl
locals {
  greeting = random([for g in ["hello", "hey", "yo"] : title(g)])
}
```

---

## 5. CLI

```
glaze up       apply the profile (creates the session)
    --var name=value    set or override a variable (repeatable)
    --var-file <path>   native HCL file of variable values
    --detached          create the session without attaching
    --clear             kill an existing same-named session first
    --debug             log every command sent to the tmux socket
    --profile-path      path to a .glaze file
glaze down     kill the session described by the profile
    --session           session to kill (skips profile resolution)
    --var / --var-file  as above (only the session name is evaluated)
    --profile-path      as above
glaze ls       list the sessions on the target tmux server
glaze format   rewrite the profile to a canonical form
    --stdout            print instead of writing
    --validate          validate first; diagnostics stop the rewrite
    --profile-path      as above
    --var / --var-file  as above
glaze save     capture a live session into a profile
    --session           session to save (the default is the current session)
    --profile-path      output path (the default is .glaze)
    --stdout            print instead of writing
```

The `up`, `down`, `ls` and `save` commands also accept `--socket-path` and `--socket-name`. These flags select a tmux server on a non-default socket. Each command accepts the global `--log-level` flag. The default log level is `info`.

Glazier resolves the profile in this order: the `--profile-path` value, then `.glaze` in the current directory, then `$GLAZE_PATH/.glaze`.

---

## 6. Diagnostics

Glazier reports errors as Terraform-style diagnostics with source snippets. The shared `diagnostics` package renders them. One run reports each declaration problem that Glazier can find, for example duplicate variables, malformed blocks and unresolvable locals. It does not stop at the first problem. Each error-severity diagnostic causes a non-zero exit code. Thus you can use `glaze format --validate` in CI.
