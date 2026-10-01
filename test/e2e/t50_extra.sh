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
  eq "failed up exits 1" 1 "$RC"
  gone "failed up removes the partly built session" half
  match "failed up says it removed the session" 'removed the partly built session' "$ERR"
  up; if [[ $RC -ne 0 ]]; then ok "retry after failed up reports a problem"; else ko "retry after failed up reports a problem" "rc=0, half-built session kept: windows=[$(wnames half)]"; fi
  up --keep-on-failure; rcnz "up --keep-on-failure still fails"
  exists "up --keep-on-failure keeps the partly built session" half
  eq "the kept session has the windows built so far" "good,bad" "$(wnames half)"
  end

  # A signal during a wait must stop glaze, remove the session it created and leave no waiting tmux client.
  local sig code i pid
  for sig in TERM INT; do
    begin "up_signal_$sig"
    pane_cmds sg '["sleep 600", "true"]'
    "$G" up --detached --socket-name "$SOCK" >"$WD/out" 2>&1 &
    pid=$!
    for ((i = 0; i < 100; i++)); do pgrep -f "wait-for glaze-" >/dev/null && break; sleep 0.1; done
    if pgrep -f "wait-for glaze-" >/dev/null; then ok "up waits for the commands [$sig]"; else ko "up waits for the commands [$sig]" "no wait-for client"; fi
    kill -s "$sig" "$pid"; wait "$pid"; code=$?
    eq "up exits 128 + SIG$sig" "$([[ $sig == TERM ]] && echo 143 || echo 130)" "$code"
    gone "SIG$sig removes the partly built session" sg
    eq "SIG$sig leaves no wait-for client" "" "$(pgrep -f 'wait-for glaze-')"
    match "up names the signal" "glaze stopped on SIG$sig" "$(cat "$WD/out")"
    end
  done
}
