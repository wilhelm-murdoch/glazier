# Variables, locals, functions, idempotence, down, ls, format.

var_profile() {
  fx <<'EOF'
variable "district" {
  description = "d"
  type        = string
  default     = "watson"
}

variable "fixer" {
  type = string
}

variable "count" {
  type    = number
  default = 2
}

variable "loud" {
  type    = bool
  default = false
}

session {
  name = "gig-${var.district}"
  window {
    name = "${var.fixer}-${var.count}-${var.loud}"
    pane {}
  }
}
EOF
}

t_variables() {
  begin var_default_and_flag
  var_profile; up --var fixer=wakako; rc0 "up with required var"
  exists "default applied" gig-watson; eq "flag var used" "wakako-2-false" "$(wnames gig-watson)"
  up --var fixer=rogue --var district=arasaka --var count=7 --var loud=true; rc0 "override all"
  eq "overrides applied" "rogue-7-true" "$(wnames gig-arasaka)"
  end

  begin var_required_missing
  var_profile; up; rcnz "missing required var"; no_server "missing var"
  match "missing var names the variable" 'fixer' "$ERR$OUT"; end

  begin var_undeclared
  var_profile; up --var fixer=x --var nope=1; rcnz "undeclared --var rejected"; no_server "undeclared var"; end

  begin var_type_errors
  var_profile
  up --var fixer=x --var count=two; rcnz "number var given 'two'"
  up --var fixer=x --var loud=yes; rcnz "bool var given 'yes'"
  no_server "type errors"
  up --var fixer=x --var count=1.5; info "number var given 1.5" "rc=$RC windows=$(wnames gig-watson)"; end

  begin var_flag_formats
  var_profile
  up --var "fixer=a,b,c"; rc0 "comma in value"; eq "comma value kept whole" "a,b,c-2-false" "$(wnames gig-watson)"; tm kill-server
  up --var "fixer=k=v"; rc0 "= in value"; eq "= in value kept" "k=v-2-false" "$(wnames gig-watson)"; tm kill-server
  up --var "fixer="; info "empty value for required var" "rc=$RC windows=$(wnames gig-watson) err=[$ERR]"; tm kill-server
  up --var "fixer"; rcnz "--var without ="
  up --var "fixer =x"; rcnz "--var name with trailing space"
  up --var "=x"; rcnz "--var with empty name"
  end

  begin var_file
  var_profile
  printf 'fixer = "fromfile"\ndistrict = "file"\n' >vars.hcl
  up --var-file vars.hcl; rc0 "--var-file HCL"; exists "var-file beats default" gig-file; eq "var-file value" "fromfile-2-false" "$(wnames gig-file)"
  up --var-file vars.hcl --var district=flag; rc0 "--var-file plus --var"; exists "--var beats var-file" gig-flag
  printf '{"fixer": "json", "district": "json"}\n' >vars.json
  tm kill-server
  up --var-file vars.json; info "--var-file JSON (README says supported, SPEC says not)" "rc=$RC sessions=[$(tm ls -F '#S' | paste -sd, -)] err=[$ERR]"
  tm kill-server
  up --var-file missing.hcl --var fixer=x; rcnz "missing --var-file"
  printf 'fixer = "x"\nbogus = "y"\n' >bad.hcl; up --var-file bad.hcl; rcnz "undeclared name in var-file"
  printf 'fixer = \n' >broken.hcl; up --var-file broken.hcl; rcnz "syntax error in var-file"
  end

  begin var_env_namespace
  fx <<'EOF'
session {
  name = "e-${env.tok}"
  window {
    pane {}
  }
}
EOF
  GLAZE_ENV_tok=abc123 gz up --detached --socket-name "$SOCK"; rc0 "env.* from GLAZE_ENV_"; exists "env value used" e-abc123
  tm kill-server
  gz up --detached --socket-name "$SOCK"; rcnz "missing env.* reference"; no_server "missing env"
  GLAZE_ENV_TOK=upper gz up --detached --socket-name "$SOCK"; info "GLAZE_ENV_ case sensitivity (TOK vs tok)" "rc=$RC"
  end

  begin var_path_namespace
  mkdir -p "My Proj"; cd "My Proj"
  fx <<'EOF'
session {
  name = "pp"
  starting_directory = path.pwd
  window {
    name = upper(path.base)
    pane {}
  }
}
EOF
  up; rc0 "path.pwd/path.base"; eq "path.base" "MY PROJ" "$(wnames pp)"; eq "path.pwd" "$WD/My Proj" "$(ppaths =pp:)"
  end

  begin locals_and_functions
  fx <<'EOF'
variable "district" {
  default = "Night City"
}

locals {
  session = "gig-${local.slug}"
  slug    = lower(replace(var.district, " ", "-"))
  editors = ["nvim", "hx", "vim"]
  pick    = random([for e in local.editors : upper(e)])
}

session {
  name = local.session
  window {
    name = local.pick
    pane {
      name = format("%s|%d|%d|%s|%s", title("a b"), len(local.editors), strlen("abcd"), join("+", local.editors), trimspace("  x  "))
    }
    pane {
      name = regexreplace(substr(reverse("fedcba"), 1, 3), "c", "C")
    }
  }
}
EOF
  up; rc0 "up with locals and functions"; exists "chained locals, order independent" gig-night-city
  match "random picks from list" '^(NVIM|HX|VIM)$' "$(wnames gig-night-city)"
  eq "function results" "A B|3|4|nvim+hx+vim|x,bCd" "$(ptitles =gig-night-city:)"
  end

  begin locals_errors
  fx <<'EOF'
locals {
  a = local.b
  b = local.a
}
session {
  name = local.a
  window {
    pane {}
  }
}
EOF
  up; rcnz "circular locals"; no_server "circular locals"
  fx <<'EOF'
locals {
  a = "1"
}
locals {
  a = "2"
}
session {
  name = local.a
  window {
    pane {}
  }
}
EOF
  up; rcnz "duplicate local"
  fx <<'EOF'
session {
  name = random([])
  window {
    pane {}
  }
}
EOF
  up; rcnz "random of empty list"
  fx <<'EOF'
session {
  name = nosuchfunc("x")
  window {
    pane {}
  }
}
EOF
  up; rcnz "unknown function"
  fx <<'EOF'
variable "x" {
  default = "a"
}
variable "x" {
  default = "b"
}
session {
  name = var.x
  window {
    pane {}
  }
}
EOF
  up; rcnz "duplicate variable"
  end

  begin var_in_commands_hooks
  fx <<'EOF'
variable "msg" {
  default = "hello"
}
session {
  name = "vc"
  window {
    pane {
      commands = ["echo ${var.msg} > @WD@/o"]
    }
  }
}
EOF
  up --var msg=wired; rc0 "var in command"; wf "$WD/o"; eq "var interpolated into command" "wired" "$(cat "$WD/o" 2>/dev/null)"
  end
}

t_idempotence() {
  begin up_twice
  fx <<'EOF'
session {
  name = "id"
  commands = ["echo run >> @WD@/o"]
  window {
    name = "w1"
    pane {}
    pane {}
  }
  window {
    name = "w2"
    pane {}
  }
}
EOF
  up; rc0 "first up"; local s1; s1=$(snap id)
  up; rc0 "second up"; eq "second up leaves session untouched" "$s1" "$(snap id)"
  sleep 0.5; eq "session commands not rerun" "1" "$(wc -l <"$WD/o")"
  info "second up output" "$OUT$ERR"
  end

  begin up_clear
  fx <<'EOF'
session {
  name = "cl"
  window {
    name = "w1"
    pane {}
    pane {}
  }
}
EOF
  up; local s1; s1=$(snap cl); tm neww -t =cl: -n extra; tm splitw -t =cl:w1
  up --clear; rc0 "up --clear"
  eq "--clear rebuilds to profile" "$(echo "$s1" | cut -d'|' -f1-3 | sed 's/|[0-9]*$//')" "$(snap cl | cut -d'|' -f1-3 | sed 's/|[0-9]*$//')"
  eq "--clear window list" "w1" "$(wnames cl)"
  end

  begin up_clear_not_running
  simple cn; up --clear; rc0 "--clear with no server"; exists "created" cn; end

  begin up_clear_other_sessions
  simple cm; tm new-session -d -s bystander; up --clear; rc0 "up --clear"; exists "bystander survives --clear" bystander; end

  begin up_existing_foreign
  simple fg; tm new-session -d -s fg -n mine; up; rc0 "up against a foreign session"; eq "foreign session untouched" "mine" "$(wnames fg)"; end

  begin up_concurrent
  simple cc
  (gz up --detached --socket-name "$SOCK") & (sleep 0.01; "$G" up --detached --socket-name "$SOCK" >"$WD/o2" 2>&1); wait
  sleep 0.5; info "two concurrent ups" "sessions=[$(tm ls -F '#S' | paste -sd, -)] windows=$(wnames cc)"
  end
}

t_down() {
  begin down_basic
  simple dn; tm new-session -d -s other; up
  down; rc0 "down"; gone "down kills profile session" dn; exists "down leaves other sessions" other
  down; rc0 "down again (not running) is a no-op"
  end

  begin down_no_server
  simple dn; down; rc0 "down with no tmux server is a no-op"; info "down with no server output" "$ERR$OUT"; end

  begin down_session_flag
  mkdir -p empty; cd empty; tm new-session -d -s "tgt"; tm new-session -d -s keep
  down --session tgt; rc0 "down --session without profile"; gone "tgt killed" tgt; exists "keep survives" keep
  down --session nonexistent; rc0 "down --session unknown is a no-op"
  tm new-session -d -s "sp ace"; down --session "sp ace"; rc0 "down --session with space"; gone "spaced session killed" "sp ace"
  end

  begin down_prefix_match
  mkdir -p empty; cd empty; tm new-session -d -s "project-long"
  down --session project; rc0 "down --session prefix"; exists "prefix does not kill project-long" project-long
  end

  begin down_vars
  var_profile; up --var fixer=x --var district=pacifica; exists "up" gig-pacifica
  down; info "down without --var when name uses a default var" "rc=$RC err=[$ERR]"; exists "default-name down leaves pacifica" gig-pacifica
  down --var district=pacifica; rc0 "down with --var (deep required var missing)"; gone "interpolated session killed" gig-pacifica
  end
}

t_ls() {
  begin ls_no_server
  gz ls --socket-name "$SOCK"; rc0 "ls with no server"
  eq "ls with no server prints nothing" "" "$OUT"
  match "ls with no server says so on stderr" 'no tmux server is running' "$ERR"; end

  # nobody cannot open root's socket directory, which gives tmux "Permission denied".
  local sub
  for sub in ls down up; do
    begin "${sub}_unreachable"
    simple unr; tm new-session -d -s unr; chmod 755 "$WD"
    [[ $sub == ls ]] && r su -s /bin/sh nobody -c "$G ls --socket-path /tmp/tmux-0/$SOCK"
    [[ $sub == down ]] && r su -s /bin/sh nobody -c "$G down --session unr --socket-path /tmp/tmux-0/$SOCK --profile-path $WD/.glaze"
    [[ $sub == up ]] && r su -s /bin/sh nobody -c "$G up --detached --socket-path /tmp/tmux-0/$SOCK --profile-path $WD/.glaze"
    eq "$sub exits 4 when tmux is unreachable" 4 "$RC"
    match "$sub names the cause" 'Permission denied' "$ERR"
    exists "$sub leaves the session alone" unr
    end
  done

  begin ls_sessions
  mkdir -p a "b dir"; tm new-session -d -s alpha -c "$WD/a"; tm neww -t =alpha:; tm new-session -d -s "beta two" -c "$WD/b dir"
  gz ls --socket-name "$SOCK"; rc0 "ls"
  match "ls header" 'NAME +WINDOWS +PATH' "$OUT"
  match "ls alpha row" "alpha +2 +$WD/a" "$OUT"
  match "ls spaced name row" "beta two +1 +$WD/b dir" "$OUT"
  nomatch "no current-session marker when detached" '\*' "$OUT"
  info "ls output" "$OUT"
  end

  begin ls_socket_path
  tmux -S "$WD/sock" new-session -d -s sp
  gz ls --socket-path "$WD/sock"; rc0 "ls --socket-path"; match "ls via socket path" 'sp' "$OUT"
  tmux -S "$WD/sock" kill-server
  end
}

t_format() {
  begin fmt_stdout
  cat >.glaze <<'EOF'
# leading comment
session {
name="fm"
    window {
  name   =    "w"   # trailing comment
      pane {
commands=["a","b"]
}
}
}
EOF
  cp .glaze orig
  gz format --stdout; rc0 "format --stdout"
  eq "--stdout leaves file untouched" "$(cat orig)" "$(cat .glaze)"
  match "canonical indentation" $'\n  name = "fm"' "$OUT"
  match "comments preserved" '# leading comment' "$OUT"
  match "trailing comment preserved" '# trailing comment' "$OUT"
  nomatch "no log noise on stdout" 'INF|WRN' "$OUT"
  gz format; rc0 "format in place"
  local once; once=$(cat .glaze); gz format; eq "format is idempotent" "$once" "$(cat .glaze)"
  eq "in-place equals --stdout" "$once" "$(gz format --stdout; echo "$OUT")"
  end

  begin fmt_validate_invalid
  layout_fixture fv foo 1; cp .glaze orig
  gz format --validate; rcnz "--validate with invalid layout"; eq "file untouched on validation error" "$(cat orig)" "$(cat .glaze)"
  end

  begin fmt_syntax_error
  printf 'session {\n  name = \n' >.glaze; cp .glaze orig
  gz format; rcnz "format on syntax error"; eq "file untouched on syntax error" "$(cat orig)" "$(cat .glaze)"
  end

  begin fmt_validate_vars
  var_profile
  gz format --validate; rcnz "--validate without required var"
  gz format --validate --var fixer=x; rc0 "--validate with required var"
  gz format --validate --var fixer=x --var count=abc; rcnz "--validate with bad typed var"
  end

  begin fmt_multiple_errors
  fx <<'EOF'
session {
  name = "me"
  window {
    layout = "foo"
    pane {
      size {
        x = "abc"
      }
    }
  }
  window {
    starting_directory = "/nope"
    pane {}
  }
}
EOF
  gz format --validate; rcnz "--validate multi-error"
  local n; n=$(grep -c 'Error' <<<"$ERR$OUT")
  if ((n >= 3)); then ok "all errors reported in one run ($n)"; else ko "all errors reported in one run" "saw $n: $ERR$OUT"; fi
  nomatch "diagnostics show source snippets" 'source code not available' "$ERR$OUT"
  info "diagnostic rendering" "$ERR$OUT"
  end

  begin fmt_perms_symlink
  simple fp real.glaze; chmod 600 real.glaze; ln -s real.glaze .glaze
  printf '\n\n' >>real.glaze
  gz format; rc0 "format through symlink"
  if [[ -L .glaze ]]; then ok "symlink preserved"; else ko "symlink preserved" "format replaced the symlink with a regular file"; fi
  eq "permissions preserved" "600" "$(stat -c %a real.glaze)"
  end

  begin fmt_no_tmux_needed
  simple fn; gz format --validate; rc0 "format --validate works"; no_server "format"
  end

  begin fmt_readonly
  simple fr; printf '\n\n' >>.glaze; chmod 444 .glaze
  gz format; info "format on read-only file" "rc=$RC err=[$ERR]"
  end
}
