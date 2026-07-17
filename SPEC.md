# Glazier profile specification

This is the reference for the `.glaze` profile format: the blocks, their attributes, and the expression language they share. It complements the task-oriented walkthrough in [README.md](README.md).

A profile is written in native HCL. It contains **exactly one `session` block**, optionally alongside any number of `variable` and `locals` blocks. Only `variable` blocks take a label (the variable's name); `session`, `window` and `pane` are named with a `name` attribute instead.

---

## 1. Blocks

### 1.1 `session`

The single root block describing the tmux session.

```hcl
session {
  name               = "daemon-run"     # defaults to "default"
  starting_directory = "~/runs/arasaka" # defaults to the current directory

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

| Attribute            | Type        | Notes                                     |
| -------------------- | ----------- | ----------------------------------------- |
| `name`               | string      | session name; defaults to `default`       |
| `starting_directory` | string      | must exist; defaults to the working dir   |
| `envs`               | map(string) | environment variables for the session     |
| `hooks`              | map(string) | tmux hook name → command                  |
| `options`            | map(string) | tmux option name → value                  |
| `window`             | block(s)    | one or more windows (required)            |

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

| Attribute | Type        | Notes                                            |
| --------- | ----------- | ------------------------------------------------ |
| `name`    | string      | window name                                      |
| `layout`  | string      | a named preset or a raw tmux layout string       |
| `focus`   | bool        | make this the active window                      |
| `hooks`   | map(string) | tmux hook name → command                         |
| `options` | map(string) | tmux option name → value                         |
| `pane`    | block(s)    | one or more panes (required)                     |

`layout` defaults to `tiled`. Beyond the five presets - `even-horizontal`, `even-vertical`, `main-horizontal`, `main-vertical`, `tiled` - it also accepts a **raw tmux layout coordinate string** (e.g. `"bb62,80x24,0,0"`), which `glaze save` captures from a live window and `glaze up` replays verbatim. The value is structurally validated at parse time; a malformed string fails fast.

### 1.3 `pane`

```hcl
pane {
  name     = "breach-protocol"
  focus    = true
  commands = ["nvim ./daemons", "echo upload ready"]

  size {                     # absolute resize
    x = "60%"                # cells ("80") or percentage ("60%")
    y = "100"
  }

  adjust {                   # directional resize, up to 4 blocks, applied in order
    direction = "left"       # up | down | left | right
    amount    = "5"
  }

  options = { "remain-on-exit" = "on" }
}
```

| Attribute  | Type         | Notes                                             |
| ---------- | ------------ | ------------------------------------------------- |
| `name`     | string       | pane title                                        |
| `focus`    | bool         | make this the active pane                         |
| `commands` | list(string) | commands sent in order (see below)                |
| `options`  | map(string)  | tmux option name → value                          |
| `size`     | block        | absolute resize; `x`/`y` in cells or percent      |
| `adjust`   | block(s)     | directional resize, up to 4, applied in order     |

`commands` are serialised with `tmux wait-for`, so each finishes before the next is sent; the **final** command is fire-and-forget, so a long-running or interactive command does not block session creation. `size` is applied first, then any `adjust` blocks.

---

## 2. Variables

A profile declares its inputs with `variable` blocks. A declared variable is read through the `var.` namespace and only that namespace; supplying a value for a name no block declares is an error.

```hcl
variable "district" {
  description = "the district the gig is themed after"
  type        = string       # optional; defaults to string
  default     = "watson"     # omit ⇒ the variable is required
}
```

| Attribute     | Required | Notes                                                                 |
| ------------- | -------- | --------------------------------------------------------------------- |
| `type`        | no       | Bare keyword `string`, `number`, or `bool`. Defaults to `string`      |
| `default`     | no       | Literal, coerced to `type` (`default = 1` satisfies a `number`)       |
| `description` | no       | Literal string; documentation only                                    |

The `type` attribute is **optional and defaults to `string`**, so a profile that only injects text stays terse (`variable "fixer" {}`) while one that needs a number or bool can say so. Supplied values and defaults are coerced to the declared type; a value that cannot convert (`--var count=two` for a `number`) is rejected before anything launches.

Values are supplied by **`--var name=value`** (repeatable) or **`--var-file <path>`**. Precedence, last write wins:

```
default > --var-file > --var
```

A required variable (no default) left unset, and a `--var`/`--var-file` entry naming an undeclared variable, are both errors.

> `glaze down` evaluates only the session `name`, so a variable used solely
> deeper in the profile is neither required nor resolved when tearing a session
> down - teardown stays idempotent.

---

## 3. `locals`

Named values shared across the profile; derived strings, palettes, computed lists. Locals may reference `var.*`, `env.*`, `path.*`, the function library, and each other in any declaration order (cycles fail loudly); expressions reference them as `local.<name>`.

```hcl
locals {
  slug     = lower(replace(var.district, " ", "-"))
  session  = "gig-${local.slug}"
  editors  = ["nvim", "hx", "vim"]
}
```

Unlike variables, locals are not settable from the CLI; they exist so a value is declared once and reused, Terraform-style.

---

## 4. Expressions

Any attribute value is an HCL expression. Available context:

- `var.<name>` — declared variables.
- `local.<name>` — resolved locals.
- `env.<name>` — environment variables prefixed `GLAZE_ENV_` (the prefix is stripped: `GLAZE_ENV_token` ⇒ `env.token`).
- `path.pwd` / `path.base` — the working directory and its basename.
- String interpolation and templates: `name = "gig-${var.district}"`.
- Inline comprehensions (native HCL `for` expressions): `[for e in local.editors : upper(e)]`.

### 4.1 Function library

Thin wrappers over the [go-cty](https://github.com/zclconf/go-cty) stdlib, plus `random`:

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

`len` counts collection elements; `strlen` counts string characters. `random(list)` returns a seeded random element of a list as a string, pairing naturally with a comprehension:

```hcl
locals {
  greeting = random([for g in ["hello", "hey", "yo"] : title(g)])
}
```

---

## 5. CLI

```
glaze up       apply the profile (creates the session)
    --var name=value    set/override a variable (repeatable)
    --var-file <path>   HCL or JSON file of variable values
    --detached          create the session without attaching
    --clear             kill an existing same-named session first
    --profile-path      path to a .glaze file
glaze down     kill the session described by the profile
    --session           session to kill (skips profile resolution)
    --var / --var-file  as above (only the session name is evaluated)
glaze ls       list sessions on the target tmux server
glaze format   rewrite the profile to canonical form
    --stdout            print instead of writing
    --validate          validate first; diagnostics abort the rewrite
    --profile-path      path to a .glaze file
    --var / --var-file  as above
glaze save     capture a live session back into a profile
```

Profiles resolve from `--profile-path`, else `.glaze` in the working directory, else `$GLAZE_PATH/.glaze`.

---

## 6. Diagnostics

Errors are reported as Terraform-style diagnostics with source snippets, via the shared `diagnostics` package. A single run surfaces every declaration problem it can (duplicate variables, malformed blocks, unresolvable locals) rather than stopping at the first. Any error-severity diagnostic makes the command exit non-zero, so `glaze format --validate` is suitable for CI.
