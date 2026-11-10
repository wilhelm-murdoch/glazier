# Glazier profile specification

This document is the reference for the `.glaze` profile format. It describes the blocks, the attributes and the expression language. The [README](README.md) gives a walkthrough of the same material.

A profile is a native HCL file. A profile contains exactly one `session` block. A profile can also contain `variable` blocks and `locals` blocks at the top level. Only a `variable` block has a label. The label is the name of the variable. You name a `session`, a `window` or a `pane` with its `name` attribute. Brackets, braces, parentheses and strings can nest at most 256 levels deep in a profile or a `--var-file`.

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

  hooks   = { "client-attached" = "run-shell 'echo jacked-in'" }
  options = { "base-index" = "1" }

  window {
    # ...at least one window is required
  }
}
```

| Attribute            | Type         | Notes                                                          |
| -------------------- | ------------ | -------------------------------------------------------------- |
| `name`               | string       | The session name. The default value is `default`, except for a profile in the profile registry, which must set `name`. It must not be empty. |
| `starting_directory` | string       | The directory must exist. The default value is the current directory. |
| `commands`           | list(string) | Commands for the active pane of the session. See 1.4.          |
| `envs`               | map(string)  | Environment variables for the session.                         |
| `hooks`              | map(string)  | A map of a tmux hook name to a tmux command. The name must be one that tmux 3.2a to 3.7c knows. |
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
| `hooks`              | map(string) | A map of a tmux hook name to a tmux command. The name must be one that tmux 3.2a to 3.7c knows. |
| `options`            | map(string) | A map of a tmux option name to a value.                        |
| `pane`               | block(s)    | One or more panes. At least one pane is required.              |

The default `layout` is `tiled`. There are five presets: `even-horizontal`, `even-vertical`, `main-horizontal`, `main-vertical` and `tiled`. The attribute also accepts a raw tmux layout string, for example `"bb62,80x24,0,0"`. The `glaze save` command captures this string from a live window. The `glaze up` command replays the string without change. Glazier validates the structure of the string at parse time. A malformed string causes an error. The string must also describe as many panes as the window declares.

tmux 3.9 and later print a layout as JSON, for example `{"V":2,"L":{"t":"h","w":80,"h":24,...}}`. Glazier accepts this form too, and `glaze save` writes the form that the running tmux prints. A profile with a JSON layout needs tmux 3.9 or later. tmux 3.9 also accepts the classic string.

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
| `hooks`              | map(string)  | A map of a tmux hook name to a tmux command. The name must be one that tmux 3.2a to 3.7c knows. |
| `options`            | map(string)  | A map of a tmux option name to a value.                        |
| `size`               | block        | An absolute resize. See below.                                 |
| `adjust`             | block(s)     | A directional resize. A maximum of four blocks. See below.     |

A `size` block must contain `x`, `y` or both. A value is a count of cells, for example `"80"`. A value can also be a percentage, for example `"60%"`. A count must be a positive integer.

An `adjust` block must contain `direction` and `amount`. The `direction` value is `up`, `down`, `left` or `right`. The `amount` value is 1 or more cells. It cannot be a percentage. Glazier applies the `size` block first. Glazier then applies each `adjust` block in declaration order.

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

You supply values with the `--var name=value` flag or with a `--var-file <path>` flag. The `--var` flag is repeatable. A var file is a native HCL file of `name = value` attributes. Glazier does not support JSON var files.

Glazier applies values in this order: the default first, then the var file, then each `--var` flag. The last value for a name applies.

An unset required variable causes an error. A `--var` flag or a var file entry with an undeclared name causes an error.

> The `glaze down` command evaluates only the session `name`. A variable that only appears deeper in the profile is not required for teardown. Thus teardown stays idempotent.

---

## 3. `locals`

A `locals` block declares named values for use across the profile, for example derived strings or computed lists. A local can reference `var.*`, `env.*`, `path.*`, the function library and other locals. The declaration order of locals has no effect. A circular reference causes an error. A duplicate local name causes an error. All locals together can hold at most 1 MiB: the bytes of each string plus one for each element. The first local past that limit causes an error. You read a local as `local.<name>`.

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
- `reverselist`
- `split`
- `strlen`
- `substr`
- `title`
- `trim`
- `trimprefix`
- `trimspace`
- `trimsuffix`
- `upper`

The `len` function counts the elements of a collection. The `strlen` function counts the characters of a string. The `reverse` function reverses the characters of a string. The `reverselist` function reverses the order of a list, for example `reverselist(local.editors)`. The `random(list)` function returns one random element of a list as a string. The result changes between runs. An empty list causes an error. The function pairs naturally with a comprehension:

```hcl
locals {
  greeting = random([for g in ["hello", "hey", "yo"] : title(g)])
}
```

---

## 5. CLI

```
glaze up [name]         apply the profile (creates the session)
    name                a profile in the profile registry: $GLAZE_REGISTRY, else $XDG_CONFIG_HOME/glaze, else ~/.config/glaze
    --var name=value    set or override a variable (repeatable)
    --var-file <path>   native HCL file of variable values
    --detached          create the session without attaching
    --clear             kill an existing same-named session first
    --debug             log every command sent to the tmux socket
    --profile-path      path to a .glaze file (not together with a name)
glaze diff [name]       show what up --clear would change in the running session; changes nothing
    name                as above
    --quiet, -q         print nothing; the exit code carries the result
    --var / --var-file  as above (every declared variable needs a value)
    --profile-path      as above
glaze down [name]       kill the session described by the profile
    name                as above
    --session           session to kill (skips profile resolution; not together with a name)
    --var / --var-file  as above (only the session name is evaluated)
    --profile-path      as above
glaze ls                list the sessions on the target tmux server
    --profiles          list the profiles in the profile registry instead of the sessions
glaze format [name]     rewrite the profile to a canonical form
    name                as above
    --stdout            print instead of writing
    --validate          validate first; diagnostics stop the rewrite
    --profile-path      as above
    --var / --var-file  as above
glaze save [name]       capture a live session into a profile
    name                a profile to write in the profile registry (as above); creates the profile registry when it is missing
    --session           session to save (the default is the current session; combines with a name)
    --profile-path      output path (the default is .glaze; not together with a name)
    --stdout            print instead of writing
    --force             replace the target profile when it exists
```

The `up`, `diff`, `down`, `ls` and `save` commands also accept `--socket-path` and `--socket-name`. These flags select a tmux server on a non-default socket. Each command accepts the global `--log-level` flag. The default log level is `info`.

With a profile name, Glazier uses `<name>.glaze` in the profile registry and nothing else. `save` with a profile name writes that file and creates the profile registry when it is missing. A profile in the profile registry must set the session `name`. Without it, `up`, `diff`, `down` and `format --validate` stop with exit code `3`. Without a profile name, Glazier resolves the profile in this order: the `--profile-path` value, then `.glaze` in the current directory, then `$GLAZE_PATH/.glaze`.

### 5.1 The report of `glaze diff`

`glaze diff` writes one report to stdout, after it read the whole session, and changes nothing. The report has three forms. For a session that does not run, every window and pane is a `+` line, and the exit code is `5`. For a session that runs and shows no difference, the report is the sentence `No changes. Session "<name>" matches the profile in everything glaze can compare.`, and the exit code is `0`. Otherwise the report is a tree with two spaces per level. It lists the session, each window with its position in `[n]` and each pane with its position. A window or pane without a difference is left out. The marks are:

- `+`: a window or pane that the profile has and the session does not have. The second column is the directory of the pane, or `(current directory)` for a pane that starts where `up` runs.
- `-`: a window or pane that the session has and the profile does not have. A window that tmux names after its program ends with `(named by tmux)`. A pane with the default title of tmux prints `(unnamed)` in place of its name.
- `~`: a value that differs and that `up --clear` sets. The line shows the live value, then `->`, then the profile value.
- `?`: a value that differs, but tmux cannot tell if Glazier set it or a user changed it. The line shows the live value, then the profile value, then `;` and the reason.

The matching rules are:

1. Windows match by name, after the same sanitization as `up`, in order. A renamed window is one `-` line and one `+` line.
2. A profile window with `automatic-rename` on, in its own options or in the options of the session, matches the next live window that tmux named. Its name is not compared.
3. Panes match by position in their window.
4. A pane at a matched position with the host name as its title is a pane that Glazier did not create. Glazier titles every pane it creates. It is a `~` line.
5. The order of the matched windows is a `~` line when it differs.
6. The focus is compared only where the profile declares `focus = true`. The last declaration wins, as in `up`. It is a `?` line.
7. The session `starting_directory` is a `~` line when the profile declares it. When the profile does not declare it, a difference from the current directory is a `?` line.
8. The current directory of a pane and a pane title that differ are `?` lines.

The `Plan` line counts the items: `Plan: A to add, C to change, R to remove.`, with `, K to check` before the full stop when K is greater than 0. A `+` or `-` item counts with each of its children. An item with a `~` line counts once as a change. An item with only `?` lines counts once to check. Two fixed lines follow: `Not compared: commands, envs, hooks and options. tmux does not report what glaze set.` and `Not compared: size, adjust and layout. tmux reports them in a form that changes with the terminal size.` A hint ends the report when there is work for `up` or `up --clear`. Exit code `5` means a counted difference or a session that does not run. `?` lines alone give exit code `0`. With `--quiet`, the command prints nothing.

---

## 6. Diagnostics

Glazier reports errors as Terraform-style diagnostics with source snippets. The shared `diagnostics` package renders them. One run reports each declaration problem that Glazier can find, for example duplicate variables, malformed blocks and unresolvable locals. It does not stop at the first problem. Each error-severity diagnostic causes a non-zero exit code. Thus you can use `glaze format --validate` in CI.
