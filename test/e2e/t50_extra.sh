# Target ambiguity and diagnostic rendering.

t_extra() {
  begin name_collision
  fx <<'X'
session {
  name = "dev"
  window {
    name = "dev"
    pane {}
  }
  window {
    name = "logs"
    pane {}
  }
}
X
  up; rc0 "session and first window share a name"; eq "both windows created" "dev,logs" "$(wnames dev)"
  end

  begin name_collision_other_session
  tm new-session -d -s web
  fx <<'X'
session {
  name = "api"
  window {
    name = "web"
    pane {}
  }
  window {
    name = "db"
    pane {}
  }
}
X
  up; rc0 "window named after another session"; eq "api windows" "web,db" "$(wnames api)"; eq "web session untouched" "1" "$(tm lsw -t =web | wc -l)"
  end

  local n
  for n in 1 42 0; do
    begin numeric_session
    fx <<X
session {
  name = "$n"
  window {
    name = "a"
    pane {}
    pane {}
  }
  window {
    name = "b"
    pane {}
  }
}
X
    tm new-session -d -s other; tm neww -t =other:
    up; rc0 "numeric session name [$n]"; eq "numeric session [$n] windows" "a,b" "$(wnames "$n")"
    eq "other session untouched by numeric [$n]" "2" "$(tm lsw -t =other | wc -l)"
    end
  done

  begin up_diagnostics
  layout_fixture dg foo 1
  up; rcnz "invalid profile"
  nomatch "up diagnostics show source snippet" 'source code not available' "$OUT$ERR"
  nomatch "no ANSI escapes in piped diagnostics" $'\e\\[' "$OUT$ERR"
  if [[ -n $OUT && $OUT == *Error* ]]; then ko "diagnostics go to stderr" "diagnostics written to stdout"; else ok "diagnostics go to stderr"; fi
  info "up diagnostics" "$OUT$ERR"
  end

  begin color_terminal
  # script runs glaze on a pseudo-terminal, where diagnostics are coloured unless NO_COLOR is set.
  layout_fixture ct foo 1
  local tty_out; tty_out=$(script -qec "$G format --validate" /dev/null 2>&1)
  match "a terminal gets colour" $'\e\\[' "$tty_out"
  tty_out=$(NO_COLOR=1 script -qec "$G format --validate" /dev/null 2>&1)
  nomatch "NO_COLOR turns colour off on a terminal" $'\e\\[' "$tty_out"
  end
}

t_extra2() {
  begin up_prefix_existing
  simple project; tm new-session -d -s project-long
  up; rc0 "up [project] while [project-long] runs"; exists "project created" project
  end

  begin up_retry_after_failure
  fx <<'X'
session {
  name = "half"
  window {
    name = "good"
    pane {}
  }
  window {
    name = "bad"
    options = {
      "automatic-rename" = "maybe"
    }
    pane {}
  }
}
X
  up; rcnz "first up fails"
  up; if [[ $RC -ne 0 ]]; then ok "retry after failed up reports a problem"; else ko "retry after failed up reports a problem" "rc=0, half-built session kept: windows=[$(wnames half)]"; fi
  end
}
