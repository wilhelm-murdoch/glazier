# Save, hostile names, sockets, attach, scale, malformed profiles.

save_rich() {
  mkdir -p d/a d/b
  fx "${1:-.glaze}" <<'EOF'
session {
  name = "sv"
  starting_directory = "@WD@/d"
  envs = {
    SECRET = "hunter2"
  }
  hooks = {
    "session-renamed" = "display hi"
  }
  options = {
    "history-limit" = "4242"
  }
  commands = ["echo sess"]
  window {
    name = "edit"
    layout = "main-vertical"
    pane {
      name = "a"
      commands = ["echo a"]
      options = {
        "remain-on-exit" = "on"
      }
    }
    pane {
      name = "b"
      focus = true
      starting_directory = "@WD@/d/a"
    }
    pane {
      name = "c"
      starting_directory = "@WD@/d/b"
    }
  }
  window {
    name = "run"
    focus = true
    layout = "even-vertical"
    pane {
      name = "d"
    }
    pane {
      name = "e"
      focus = true
    }
  }
  window {
    name = "solo"
    pane {
      name = "f"
    }
  }
}
EOF
}

t_save() {
  begin save_roundtrip
  save_rich; up; rc0 "up rich profile"; sleep 0.3
  local s1; s1=$(snap sv)
  gz save --session sv --profile-path saved.glaze --socket-name "$SOCK"; rc0 "save to file"
  cp saved.glaze "$OUT_DIR/logs/saved_roundtrip.glaze"
  gz format --validate --profile-path saved.glaze; rc0 "saved profile validates"
  down; up --profile-path saved.glaze; rc0 "up from saved profile"; sleep 0.3
  eq "round-trip structure, focus, paths and geometry" "$s1" "$(snap sv)"
  for k in commands envs hooks options hunter2 SECRET; do nomatch "save excludes $k" "$k" "$(cat saved.glaze)"; done
  end

  begin save_stdout_clean
  save_rich; up
  gz save --session sv --stdout --socket-name "$SOCK"; rc0 "save --stdout"
  nomatch "save --stdout has no log lines on stdout" 'INF|WRN|EXPERIMENTAL' "$OUT"
  match "save --stdout emits HCL" '^session \{' "$OUT"
  [[ -e .glaze.new || $(ls -a | grep -c glaze) -gt 1 ]] && info "files after --stdout" "$(ls -a)"
  echo "$OUT" >piped.glaze; gz format --validate --profile-path piped.glaze; rc0 "piped save output validates"
  end

  begin save_default_overwrite
  simple ow; echo '# my hand-written profile' >>.glaze; up
  local before; before=$(cat .glaze)
  gz save --session ow --socket-name "$SOCK"; rc0 "save to default path"
  if [[ "$before" == "$(cat .glaze)" ]]; then ok "save does not clobber existing .glaze"; else ko "save does not clobber existing .glaze" "hand-written .glaze overwritten without prompt or backup (comment lost)"; fi
  end

  begin save_outside_tmux
  simple so; up; gz save --stdout --socket-name "$SOCK"; rcnz "save without --session outside tmux"
  match "save outside tmux asks for --session" '--session' "$ERR"
  eq "save outside tmux writes no profile" "" "$OUT"; end

  begin save_missing_session
  simple sm; up; gz save --session nope --stdout --socket-name "$SOCK"; rcnz "save of unknown session"; end

  begin save_prefix_session
  tm new-session -d -s "project-long"; gz save --session project --stdout --socket-name "$SOCK"
  if [[ $RC -eq 0 ]]; then ko "save --session does not prefix-match" "saved [$(grep -m1 name <<<"$OUT")]"; else ok "save --session does not prefix-match"; fi
  end

  begin save_no_server
  gz save --session x --stdout --socket-name "$SOCK"; rcnz "save with no server"; end

  begin save_foreign_session
  mkdir -p fdir; tm new-session -d -s raw -c "$WD/fdir"; tm splitw -h -t =raw:; tm neww -t =raw: -n two
  local s1; s1=$(snap raw | cut -d'|' -f1,3- | sed 's/^W [0-9]*/W/')
  gz save --session raw --profile-path raw.glaze --socket-name "$SOCK"; rc0 "save non-glaze session"
  cp raw.glaze "$OUT_DIR/logs/saved_foreign.glaze"
  tm kill-session -t =raw; up --profile-path raw.glaze; rc0 "up from foreign save"
  eq "foreign round-trip (ignoring titles)" "$s1" "$(snap raw | cut -d'|' -f1,3- | sed 's/^W [0-9]*/W/')"
  end

  begin save_escaping
  fx <<'EOF'
session {
  name = "esc"
  window {
    name = "q\"uote"
    pane {
      name = "$${dollar}"
    }
    pane {
      name = "%%{pct}"
    }
    pane {
      name = "back\\slash"
    }
  }
}
EOF
  # glaze replaces a backslash in a pane name with -, because tmux 3.7 and later store it escaped.
  up; rc0 "up with escape-worthy names"
  eq "window name literal" 'q"uote' "$(wnames esc)"; eq "pane names literal" '${dollar},%{pct},back-slash' "$(ptitles =esc:)"
  gz save --session esc --profile-path s.glaze --socket-name "$SOCK"; rc0 "save escape-worthy names"
  cp s.glaze "$OUT_DIR/logs/saved_escaping.glaze"
  down; up --profile-path s.glaze; rc0 "up from saved escaped profile"
  eq "escaped window name round-trips" 'q"uote' "$(wnames esc)"; eq "escaped pane names round-trip" '${dollar},%{pct},back-slash' "$(ptitles =esc:)"
  end

  begin save_titles_changed
  fx <<'EOF'
session {
  name = "tc"
  window {
    name = "w"
    pane {
      name = "mine"
      commands = ["printf '\\033]2;%s\\033\\\\' changed-by-app"]
    }
  }
}
EOF
  up; sleep 0.5; info "pane title after app sets it" "$(ptitles =tc:w)"
  end

  begin save_deleted_dir
  mkdir -p gonedir; tm new-session -d -s dd -c "$WD/gonedir"; rmdir gonedir
  gz save --session dd --profile-path dd.glaze --socket-name "$SOCK"; info "save of pane whose cwd was deleted" "rc=$RC"
  gz format --validate --profile-path dd.glaze; info "validate that saved profile" "rc=$RC err=[$ERR]"
  end
}

t_hostile_names() {
  local n
  for n in "my session" "a.b" "a:b" "semi;colon" "semi;" "hash#tag" "ünïcødé" "-dash" "pct%s" "brace{x}" "tab	x" "x=y" "$(printf 'a%.0s' {1..200})"; do
    begin hostile_session
    fx <<EOF
session {
  name = "$n"
  commands = ["echo ok > @WD@/o", "echo ok2 >> @WD@/o"]
  window {
    name = "w"
    pane {
      name = "p"
    }
    pane {
      name = "q"
    }
  }
}
EOF
    TO=10 up
    # glaze replaces the characters that tmux rewrites in a session name with -.
    local want="${n//[.:\\\$]/-}" tmuxname; tmuxname=$(tm ls -F '#S')
    if [[ "$want" != "$n" ]]; then match "warns about the renamed session [${n:0:20}]" "replacing them with hyphens" "$OUT$ERR"; fi
    if [[ $RC -eq 0 && "$tmuxname" == "$want" ]]; then ok "session name [${n:0:20}] works"
    else ko "session name [${n:0:20}] works" "rc=$RC tmux has [${tmuxname:0:40}] windows=[$(tm lsw -a -F '#W' | paste -sd, -)] err=[${ERR:0:300}]"; fi
    if [[ $RC -eq 0 ]]; then
      TO=10 up; if [[ $RC -eq 0 && $(tm ls | wc -l) -eq 1 ]]; then ok "second up idempotent [${n:0:20}]"; else ko "second up idempotent [${n:0:20}]" "rc=$RC sessions=[$(tm ls -F '#S' | paste -sd'|' -)] err=[${ERR:0:200}]"; fi
      down; if [[ -z $(tm ls -F '#S' 2>/dev/null) ]]; then ok "down removes [${n:0:20}]"; else ko "down removes [${n:0:20}]" "rc=$RC left=[$(tm ls -F '#S')] err=[${ERR:0:200}]"; fi
    fi
    end
  done

  begin hostile_empty_name
  fx <<'EOF'
session {
  name = ""
  window {
    name = ""
    pane {
      name = ""
    }
  }
}
EOF
  up; info "empty names" "rc=$RC sessions=[$(tm ls -F '#S' | paste -sd, -)] err=[$ERR]"; end

  for n in "w;1" "w;" "w:1" "w.1" "w 1" "#[fg=red]x" "#{session_name}" "ü"; do
    begin hostile_window
    fx <<EOF
session {
  name = "hw"
  window {
    name = "$n"
    layout = "even-horizontal"
    focus = true
    options = {
      "automatic-rename" = "off"
    }
    pane {
      name = "p"
    }
    pane {
      name = "q"
    }
  }
  window {
    name = "other"
    pane {}
  }
}
EOF
    up
    if [[ $RC -eq 0 && "$(tm lsw -t =hw -F '#{window_name}' | head -1)" == "$n" && $(tm lsw -t =hw | wc -l) -eq 2 ]]; then ok "window name [$n] works"
    else ko "window name [$n] works" "rc=$RC windows=[$(tm lsw -t =hw -F '#{window_name}' | paste -sd'|' -)] err=[${ERR:0:300}]"; fi
    if [[ $RC -eq 0 ]]; then
      gz save --session hw --stdout --socket-name "$SOCK"; if [[ $RC -eq 0 ]]; then ok "save with window [$n]"; else ko "save with window [$n]" "rc=$RC err=[${ERR:0:200}]"; fi
    fi
    end
  done

  for n in "p;1" "p;" "p|1" "p 1" "#{pane_id}"; do
    begin hostile_pane
    fx <<EOF
session {
  name = "hp"
  window {
    name = "w"
    pane {
      name = "$n"
      focus = true
    }
    pane {
      name = "z"
    }
  }
}
EOF
    up
    if [[ $RC -eq 0 && "$(tm lsp -t =hp:w -F '#{pane_title}' | head -1)" == "$n" ]]; then ok "pane name [$n] works"; else ko "pane name [$n] works" "rc=$RC titles=[$(tm lsp -t =hp:w -F '#{pane_title}' | paste -sd'|' -)] err=[${ERR:0:300}]"; fi
    if [[ $RC -eq 0 ]]; then
      gz save --session hp --profile-path s.glaze --socket-name "$SOCK"
      if [[ $RC -eq 0 ]] && grep -qF "focus" s.glaze; then ok "save with pane [$n]"; else ko "save with pane [$n]" "rc=$RC err=[${ERR:0:200}] out=[$(tr '\n' ' ' <s.glaze 2>/dev/null)]"; fi
    fi
    end
  done

  for n in "semi;dir" "dir;" "sp ace" "ünï" "quote'd"; do
    begin hostile_dir
    mkdir -p "$WD/$n"
    fx <<EOF
session {
  name = "hd"
  starting_directory = "@WD@/$n"
  window {
    name = "w"
    pane {
      starting_directory = "@WD@/$n"
    }
    pane {
      starting_directory = "@WD@/$n"
    }
  }
}
EOF
    up
    if [[ $RC -eq 0 ]]; then ok "dir [$n] up"; else ko "dir [$n] up" "rc=$RC err=[${ERR:0:300}]"; fi
    eq "dir [$n] paths" "$WD/$n,$WD/$n" "$(ppaths =hd:w)"
    gz ls --socket-name "$SOCK"; match "ls with dir [$n]" "hd +1 +$WD/$n" "$OUT"
    gz save --session hd --profile-path s.glaze --socket-name "$SOCK"
    if [[ $RC -eq 0 ]] && grep -qF "$WD/$n" s.glaze; then ok "save with dir [$n]"; else ko "save with dir [$n]" "rc=$RC err=[${ERR:0:200}] file=[$(tr '\n' ' ' <s.glaze 2>/dev/null)]"; fi
    end
  done
}

t_sockets() {
  begin socket_path
  simple spth
  gz up --detached --socket-path "$WD/s.sock"; rc0 "up --socket-path"
  tmux -S "$WD/s.sock" has-session -t =spth 2>/dev/null && ok "session on socket path" || ko "session on socket path" "missing"
  gz ls --socket-path "$WD/s.sock"; match "ls --socket-path" spth "$OUT"
  gz save --session spth --stdout --socket-path "$WD/s.sock"; rc0 "save --socket-path"
  gz down --socket-path "$WD/s.sock"; rc0 "down --socket-path"
  tmux -S "$WD/s.sock" has-session -t =spth 2>/dev/null && ko "down via socket path" "still there" || ok "down via socket path"
  tmux -S "$WD/s.sock" kill-server 2>/dev/null
  end

  begin socket_both
  simple sb
  gz up --detached --socket-path "$WD/s.sock" --socket-name "$SOCK"
  info "both --socket-path and --socket-name" "rc=$RC on_path=$(tmux -S "$WD/s.sock" ls -F '#S' 2>/dev/null) on_name=$(tm ls -F '#S') err=[$ERR]"
  tmux -S "$WD/s.sock" kill-server 2>/dev/null
  end

  begin socket_default_untouched
  simple sd; up
  if tmux ls >/dev/null 2>&1; then ko "--socket-name does not touch default server" "default server has sessions"; else ok "--socket-name does not touch default server"; fi
  end
}

# attach_client SESSION_CMD runs CMD on a pseudo-TTY in the background.
attach_client() {
  (sleep 30) | script -qfec "stty cols 120 rows 40; $1" "$WD/tty.log" >/dev/null 2>&1 &
  CLIENT_PID=$!
}
wait_client() { # SESSION
  local i
  for ((i = 0; i < 50; i++)); do
    [[ "$(tm lsc -F '#{client_session}' | head -1)" == "$1" ]] && return 0; sleep 0.1
  done
  return 1
}

t_attach() {
  begin attach_new
  simple at
  attach_client "$G up --socket-name $SOCK; echo GLAZE_RC=\$? > $WD/rc"
  if wait_client at; then ok "up (attached) attaches a client"; else ko "up (attached) attaches a client" "clients=[$(tm lsc -F '#{client_session}')] tty=[$(tail -c 300 "$WD/tty.log" | tr -cd '[:print:]')]"; fi
  tm detach-client -s =at; sleep 1
  eq "up exits 0 after detach" "GLAZE_RC=0" "$(cat "$WD/rc" 2>/dev/null)"
  end

  begin attach_existing
  simple ae; up
  attach_client "$G up --socket-name $SOCK"
  if wait_client ae; then ok "up attaches to an existing session"; else ko "up attaches to an existing session" "no client"; fi
  end

  begin attach_no_tty
  simple nt; gz up --socket-name "$SOCK"
  info "up without --detached and no TTY" "rc=$RC session_exists=$(has nt && echo yes || echo no) windows=$(wnames nt) err=[${ERR:0:300}]"
  end

  begin inside_tmux
  simple inner inner.glaze
  mkdir -p hostdir; tm new-session -d -s host -c "$WD/hostdir"
  attach_client "tmux -L $SOCK attach -t =host"
  if wait_client host; then ok "host client attached"; else ko "host client attached" "no client"; fi
  tm send-keys -t =host: "$G ls --socket-name $SOCK > $WD/ls.txt 2>&1" Enter
  wf "$WD/ls.txt" 5; match "ls marks the current session with *" 'host\*' "$(cat "$WD/ls.txt" 2>/dev/null)"
  tm send-keys -t =host: "$G save --stdout --socket-name $SOCK > $WD/save.txt 2>/dev/null" Enter
  wf "$WD/save.txt" 5; match "save inside tmux captures current session" 'name += "host"' "$(cat "$WD/save.txt" 2>/dev/null)"
  tm send-keys -t =host: "$G up --socket-name $SOCK --profile-path $WD/inner.glaze > $WD/up.txt 2>&1; echo RC=\$? >> $WD/up.txt" Enter
  sleep 2
  eq "up inside tmux switches the client" "inner" "$(tm lsc -F '#{client_session}' | head -1)"
  info "up inside tmux output" "$(cat "$WD/up.txt" 2>/dev/null)"
  tm send-keys -t =host: "cd $WD && $G up --socket-name $SOCK --profile-path inner.glaze > $WD/up2.txt 2>&1; echo RC=\$? >> $WD/up2.txt" Enter
  end

  begin inside_tmux_nested_default
  # glaze run inside a pane with no socket flags should target the pane's own server.
  simple nd nd.glaze; tm new-session -d -s host
  tm send-keys -t =host: "$G up --detached --profile-path $WD/nd.glaze > $WD/o 2>&1; echo RC=\$? >> $WD/o" Enter
  wf "$WD/o" 5; sleep 1
  if has nd; then ok "glaze inside a pane without socket flags targets the enclosing server"; else ko "glaze inside a pane without socket flags targets the enclosing server" "session not on enclosing server; out=[$(cat "$WD/o")]"; fi
  tmux kill-server 2>/dev/null
  end

  begin inside_other_server
  # glaze runs in a pane of the host server and targets a different server.
  local hs="${SOCK}h" i
  simple os os.glaze
  tmux -L "$hs" -f /dev/null new-session -d -s host
  attach_client "tmux -L $hs attach -t =host"
  for ((i = 0; i < 50; i++)); do [[ -n "$(tmux -L "$hs" lsc 2>/dev/null)" ]] && break; sleep 0.1; done
  tmux -L "$hs" send-keys -t =host: "$G up --socket-name $SOCK --profile-path $WD/os.glaze > $WD/up.txt 2>&1; echo RC=\$? >> $WD/up.txt" Enter
  wrc "$WD/up.txt"
  match "up inside another server exits 0" 'RC=0' "$(cat "$WD/up.txt" 2>/dev/null)"
  exists "up inside another server creates the session" os
  match "up inside another server shows the attach command" "attach -t '=os'" "$(cat "$WD/up.txt" 2>/dev/null)"
  eq "up inside another server leaves the host client alone" "host" "$(tmux -L "$hs" lsc -F '#{client_session}' 2>/dev/null | head -1)"
  eq "up inside another server attaches no client" "" "$(tm lsc)"
  tmux -L "$hs" send-keys -t =host: "$G ls --socket-name $SOCK > $WD/ls.txt 2>&1; echo RC=\$? >> $WD/ls.txt" Enter
  wrc "$WD/ls.txt"
  nomatch "ls inside another server marks no session" '\*' "$(cat "$WD/ls.txt" 2>/dev/null)"
  tmux -L "$hs" send-keys -t =host: "$G save --stdout --socket-name $SOCK > $WD/save.txt 2>&1; echo RC=\$? >> $WD/save.txt" Enter
  wrc "$WD/save.txt"
  match "save inside another server asks for --session" '--session' "$(cat "$WD/save.txt" 2>/dev/null)"
  match "save inside another server fails" 'RC=1' "$(cat "$WD/save.txt" 2>/dev/null)"
  tmux -L "$hs" kill-server 2>/dev/null
  end

  begin clear_inside_target
  simple self self.glaze; up --profile-path self.glaze
  attach_client "tmux -L $SOCK attach -t =self"
  if wait_client self; then ok "client attached to the target"; else ko "client attached to the target" "no client"; fi
  tm send-keys -t =self: "$G up --clear --socket-name $SOCK --profile-path $WD/self.glaze > $WD/clear.txt 2>&1; echo RC=\$? >> $WD/clear.txt" Enter
  if wrc "$WD/clear.txt"; then ok "--clear inside the target does not end the shell that runs it"; else ko "--clear inside the target does not end the shell that runs it" "no exit code; out=[$(cat "$WD/clear.txt" 2>/dev/null)]"; fi
  match "--clear inside the target fails" 'RC=1' "$(cat "$WD/clear.txt" 2>/dev/null)"
  match "--clear inside the target says why" 'would also end glaze' "$(cat "$WD/clear.txt" 2>/dev/null)"
  exists "--clear inside the target keeps the session" self
  end
}

t_scale() {
  local n
  for n in 6 8 12; do
    begin "scale_panes_$n"
    layout_fixture sp tiled "$n"; up
    if [[ $RC -eq 0 && $(tm lsp -t =sp:w | wc -l) -eq $n ]]; then ok "$n panes in one window (80x24)"; else ko "$n panes in one window (80x24)" "rc=$RC panes=$(tm lsp -t =sp:w 2>/dev/null | wc -l) err=[${ERR:0:300}]"; fi
    info "$n panes timing" "${DUR}ms session_left=$(has sp && echo yes || echo no)"
    end
  done

  begin scale_windows
  {
    echo 'session {'; echo '  name = "sw"'
    for i in $(seq 1 20); do echo '  window {'; echo "    name = \"w$i\""; echo '    pane {'; echo '      commands = ["true", "true"]'; echo '    }'; echo '    pane {}'; echo '  }'; done
    echo '}'
  } >.glaze
  up; rc0 "20 windows x 2 panes"; eq "20 windows" "20" "$(tm lsw -t =sw | wc -l)"; info "20 windows timing" "${DUR}ms"
  end
}

t_malformed() {
  local body
  while IFS= read -r -d $'\0' body; do
    begin malformed
    printf '%s\n' "$body" >.glaze
    up; rcnz "malformed: $(echo "$body" | tr '\n' ' ' | cut -c1-60)"; no_server "malformed"
  done < <(printf '%s\0' \
    '' \
    'session {}' \
    'session {
  window {}
}' \
    'session {
  window {
    pane {}
  }
}
session {
  window {
    pane {}
  }
}' \
    'session "labelled" {
  window {
    pane {}
  }
}' \
    'session {
  bogus = 1
  window {
    pane {}
  }
}' \
    'session {
  window {
    pane {
      focus = "notbool"
    }
  }
}' \
    'window {
  pane {}
}' \
    'session {
  name = ["list"]
  window {
    pane {}
  }
}' \
    'session {
  window {
    pane {
      commands = "not a list"
    }
  }
}')
}
