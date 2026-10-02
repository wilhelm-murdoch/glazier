# Size and adjust, commands, hooks, options and envs.

size_fixture() { # LAYOUT_LINE PANE0_BODY
  {
    echo 'session {'; echo '  name = "sz"'; echo '  window {'; echo '    name = "w"'
    [[ -n $1 ]] && echo "    layout = \"$1\""
    echo '    pane {'; echo '      name = "p0"'; printf '%s\n' "$2"; echo '    }'
    echo '    pane {'; echo '      name = "p1"'; echo '    }'
    echo '  }'; echo '}'
  } >.glaze
}
pw() { tm lsp -t =sz:w -F '#{pane_title} #{pane_width} #{pane_height}' | awk -v n="$1" '$1==n{print $2"x"$3}'; }

t_size() {
  # even-horizontal puts the panes side by side, so x applies.
  begin size_cells_even-horizontal
  size_fixture even-horizontal '      size {
        x = "20"
      }'
  up; rc0 "up"
  match "size x=20 honoured (layout=even-horizontal)" '^20x' "$(pw p0)"
  end

  begin size_pct_even-horizontal
  size_fixture even-horizontal '      size {
        x = "25%"
      }'
  up; rc0 "up"
  match "size x=25% honoured (layout=even-horizontal, ~20 cols)" '^(19|20|21)x' "$(pw p0)"
  end

  # The default tiled layout stacks two panes, so only y can change.
  begin size_cells_nolayout
  size_fixture "" '      size {
        y = "5"
      }'
  up; rc0 "up"
  match "size y=5 honoured (layout=default)" 'x5$' "$(pw p0)"
  end

  begin size_pct_nolayout
  size_fixture "" '      size {
        y = "25%"
      }'
  up; rc0 "up"
  match "size y=25% honoured (layout=default, ~6 rows)" 'x(5|6|7)$' "$(pw p0)"
  end

  begin size_y
  size_fixture even-vertical '      size {
        y = "5"
      }'
  up; rc0 "up"; match "size y=5 honoured" 'x5$' "$(pw p0)"; end

  begin adjust_right
  size_fixture even-horizontal '      adjust {
        direction = "right"
        amount = "10"
      }'
  up; rc0 "up"; info "adjust right 10 width (even split is 40)" "$(pw p0)"
  match "adjust right 10 widens p0 beyond even split" '^(49|50|51)x' "$(pw p0)"
  end

  begin adjust_order
  size_fixture even-horizontal '      size {
        x = "30"
      }
      adjust {
        direction = "right"
        amount = "5"
      }
      adjust {
        direction = "left"
        amount = "2"
      }'
  up; rc0 "up"; match "size 30 then +5 -2 gives 33" '^33x' "$(pw p0)"; end

  begin adjust_unknown_direction
  size_fixture "" '      adjust {
        direction = "unknown"
        amount = "5"
      }'
  up; rcnz "adjust direction 'unknown' rejected"; no_server "unknown direction"
  match "unknown direction diagnostic" 'not supported' "$ERR"; end

  begin adjust_bad_direction
  size_fixture "" '      adjust {
        direction = "sideways"
        amount = "5"
      }'
  up; rcnz "adjust direction 'sideways' rejected"; no_server "bad direction"; end

  begin adjust_five_blocks
  local blk='      adjust {
        direction = "up"
        amount = "1"
      }'
  size_fixture "" "$blk"$'\n'"$blk"$'\n'"$blk"$'\n'"$blk"$'\n'"$blk"
  up; rcnz "five adjust blocks rejected"; no_server "five adjust"; end

  local v
  for v in "0" "-5" "abc" "101%" "0%" "" " 10" "10.5" "1e3" "%"; do
    begin size_invalid
    size_fixture "" "      size {
        x = \"$v\"
      }"
    up; rcnz "size x=[$v] rejected"; no_server "size x=[$v]"
    end
  done

  for v in "0" "-3" "abc" "10%"; do
    begin adjust_amount_invalid
    size_fixture "" "      adjust {
        direction = \"left\"
        amount = \"$v\"
      }"
    up; rcnz "adjust amount=[$v] rejected"
    end
  done

  # A raw layout string fixes the size of every pane, so glaze ignores size and warns.
  begin size_raw_layout
  tm new-session -d -s ref; tm splitw -h -t =ref:
  local raw want; raw=$(tm display -p -t ref: '#{window_layout}'); tm kill-server
  want=$(sed -E 's/.*\{([0-9]+)x.*/\1/' <<<"$raw")
  size_fixture "$raw" '      size {
        x = "20"
      }'
  up; rc0 "up with a raw layout and a size"
  match "the raw layout keeps its width" "^${want}x" "$(pw p0)"
  match "up warns that it ignores size" 'ignores size and adjust' "$ERR"; end

  begin size_empty_block
  size_fixture "" '      size {
      }'
  up; rcnz "empty size block rejected"; no_server "empty size"; end

  begin size_huge
  size_fixture even-horizontal '      size {
        x = "5000"
      }'
  up; info "size larger than window" "rc=$RC geo=$(pw p0) err=[$ERR]"; end
}

pane_cmds() { # SESSION_NAME HCL_LIST
  fx <<EOF
session {
  name = "$1"
  window {
    name = "w"
    pane {
      name = "p"
      commands = $2
    }
  }
}
EOF
}

t_commands() {
  begin cmd_serial
  pane_cmds cs '["sleep 2; echo a >> @WD@/o", "echo b >> @WD@/o", "echo c >> @WD@/o"]'
  up; rc0 "up"
  if ((DUR >= 1900)); then ok "up waits for non-final commands (${DUR}ms)"; else ko "up waits for non-final commands" "returned in ${DUR}ms"; fi
  wf "$WD/o"; sleep 0.5; eq "commands ran in order" "a,b,c" "$(paste -sd, "$WD/o")"
  eq "no glaze buffer is left" "" "$(tm list-buffers)"
  end

  begin cmd_last_long_running
  pane_cmds cl '["echo x > @WD@/o", "sleep 600"]'
  TO=15 up; rc0 "up with long-running final command"
  if ((DUR < 5000)); then ok "final command is fire-and-forget (${DUR}ms)"; else ko "final command is fire-and-forget" "${DUR}ms"; fi
  end

  begin cmd_single_long_running
  pane_cmds cl '["sleep 600"]'
  TO=15 up; rc0 "up with single long-running command"; end

  begin cmd_nonfinal_long_running
  pane_cmds cn '["sleep 600", "echo x"]'
  TO=8 up; info "non-final long-running command blocks up (by design)" "rc=$RC dur=${DUR}ms"; end

  begin cmd_failing
  pane_cmds cf '["false", "(exit 3)", "echo after > @WD@/o"]'
  up; rc0 "up with a failing command"; wf "$WD/o"; eq "later commands still run" "after" "$(cat "$WD/o" 2>/dev/null)"; end

  begin cmd_exit_then_split
  fx <<'EOF'
session {
  name = "ex"
  window {
    name = "w"
    pane {
      name     = "runner"
      commands = ["true; exit"]
    }
    pane {
      name = "shell"
    }
    pane {
      name = "shell2"
    }
  }
}
EOF
  up; rc0 "a pane whose command exits does not break the next split"
  sleep 0.5; eq "the other panes keep their order" "shell,shell2" "$(ptitles =ex:w)"
  end

  begin cmd_trailing_comment
  pane_cmds cc '["echo a > @WD@/o # a comment", "echo b >> @WD@/o"]'
  TO=10 up; rc0 "non-final command with trailing # comment does not hang"
  wf "$WD/o" 3; sleep 0.3; eq "both commands ran" "a,b" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_trailing_ampersand
  pane_cmds ca '["sleep 300 &", "echo b > @WD@/o"]'
  TO=10 up; rc0 "non-final backgrounded command (&) does not hang"
  wf "$WD/o" 3; eq "command after & ran" "b" "$(cat "$WD/o" 2>/dev/null)"; end

  begin cmd_trailing_semicolon
  pane_cmds cs2 '["echo a > @WD@/o;", "echo b >> @WD@/o"]'
  TO=10 up; rc0 "non-final command ending in ; does not hang"
  wf "$WD/o" 3; sleep 0.3; eq "both ran" "a,b" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_exit
  pane_cmds ce '["exit", "echo b"]'
  TO=8 up
  if ((RC != 124 && RC != 137)); then ok "non-final 'exit' command does not hang (${DUR}ms)"; else ko "non-final 'exit' command does not hang" "timed out"; fi
  match "up warns that it stopped waiting" 'stopped waiting' "$ERR"
  info "the only pane exits, so tmux ends the session" "rc=$RC"; end

  begin cmd_command_timeout
  pane_cmds ct '["sleep 600", "echo b"]'
  TO=10 up --command-timeout 1s; rc0 "up with --command-timeout"
  if ((DUR < 5000)); then ok "up stops waiting after the timeout (${DUR}ms)"; else ko "up stops waiting after the timeout" "${DUR}ms"; fi
  match "up warns about the timeout" 'did not finish in time' "$ERR"; end

  begin cmd_history_bang
  pane_cmds hb '["echo wow!x > @WD@/o", "echo b >> @WD@/o"]'
  TO=10 up; rc0 "a command with ! does not hang"
  wf "$WD/o" 3; sleep 0.3; eq "! is literal" "wow!x,b" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_tab
  pane_cmds tb '["printf '"'"'a\tb\n'"'"' > @WD@/o", "echo end >> @WD@/o"]'
  TO=10 up; rc0 "a command with a tab does not hang"
  wf "$WD/o" 3; sleep 0.3; eq "the tab is literal" $'a\tb,end' "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_leading_dash
  pane_cmds ld '["-x 2>/dev/null; echo d > @WD@/o", "echo e >> @WD@/o"]'
  TO=10 up; rc0 "a command that starts with - is not a tmux flag"
  wf "$WD/o" 3; sleep 0.3; eq "both commands ran" "d,e" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_final_keyname
  pane_cmds fk '["echo a > @WD@/o", "Enter"]'
  TO=10 up; rc0 "up"; wf "$WD/o"; sleep 0.5
  match "a final key name runs as a command" 'Enter.*not found' "$(tm capture-pane -p -t =fk:w)"; end

  begin cmd_long_line
  local a5k; a5k=$(printf 'A%.0s' $(seq 1 5000))
  pane_cmds ll "[\"echo $a5k > @WD@/o\", \"echo end >> @WD@/o\"]"
  TO=10 up; rc0 "a 5000-character command does not hang"
  wf "$WD/o" 3; sleep 0.3; eq "the whole line ran" "$a5k,end" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_syntax_error
  pane_cmds se '["echo a > @WD@/o", "if then", "echo \"unclosed", "echo b >> @WD@/o", "echo end >> @WD@/o"]'
  TO=10 up; rc0 "a command with a syntax error does not hang"
  wf "$WD/o" 3; sleep 0.3; eq "the commands after it ran" "a,b,end" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_unset_tmux
  pane_cmds ut '["unset TMUX", "echo a > @WD@/o", "echo b >> @WD@/o"]'
  TO=10 up; rc0 "unset TMUX does not hang"
  wf "$WD/o" 3; sleep 0.3; eq "both commands ran" "a,b" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  local shell
  for shell in zsh fish dash; do
    begin "cmd_shell_$shell"
    if ! command -v "$shell" >/dev/null; then info "default shell $shell" "not installed"; end; continue; fi
    printf 'set -g default-shell %s\n' "$(command -v "$shell")" >"$HOME/.tmux.conf"
    : >"$HOME/.zshrc"
    mkdir -p d
    pane_cmds "s$shell" '["cd @WD@/d", "if then", "echo wow!x > @WD@/o", "printf '"'"'a\tb\n'"'"' >> @WD@/o # note", "-x 2>/dev/null; pwd >> @WD@/o", "echo end >> @WD@/o"]'
    TO=15 up; rc0 "up with $shell as the default shell"
    wf "$WD/o" 5; sleep 0.5
    eq "commands ran as written in $shell" $'wow!x,a\tb,'"$WD/d,end" "$(paste -sd, "$WD/o" 2>/dev/null)"
    eq "no glaze buffer is left in $shell" "" "$(tm list-buffers)"
    end
  done

  begin cmd_special_chars
  pane_cmds sc '["printf \"%s|%s|%s\\n\" \"a;b\" '"'"'c d'"'"' \"$HOME\" > @WD@/o", "echo done"]'
  up; rc0 "up"; wf "$WD/o"
  eq "quotes, ; and \$VAR pass through" "a;b|c d|$HOME" "$(cat "$WD/o" 2>/dev/null)"; end

  begin cmd_keyname_like
  pane_cmds kn '["echo Enter > @WD@/o", "echo C-c >> @WD@/o", "echo Space >> @WD@/o"]'
  up; rc0 "up"; wf "$WD/o"; sleep 0.3; eq "key-name-like words are literal" "Enter,C-c,Space" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_bare_keyname
  pane_cmds bk '["echo first > @WD@/o", "Escape", "echo third >> @WD@/o"]'
  TO=8 up; rc0 "a command that is exactly a tmux key name (Escape)"
  wf "$WD/o" 3; sleep 0.3; eq "the commands around it ran" "first,third" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_multiline
  pane_cmds ml '["echo one > @WD@/o\necho two >> @WD@/o", "echo three >> @WD@/o"]'
  TO=10 up; rc0 "a command containing a newline"
  wf "$WD/o" 3; sleep 0.3; eq "both lines and the next command ran" "one,two,three" "$(paste -sd, "$WD/o" 2>/dev/null)"; end

  begin cmd_pane_dir
  mkdir -p pd
  fx <<'EOF'
session {
  name = "pd"
  window {
    pane {
      starting_directory = "@WD@/pd"
      commands = ["pwd > @WD@/o"]
    }
  }
}
EOF
  up; rc0 "up"; wf "$WD/o"; eq "command runs in pane dir" "$WD/pd" "$(cat "$WD/o" 2>/dev/null)"; end

  begin cmd_many_panes_serial
  fx <<'EOF'
session {
  name = "mp"
  window {
    name = "w"
    pane {
      commands = ["sleep 1; echo a >> @WD@/o", "echo a2 >> @WD@/o"]
    }
    pane {
      commands = ["echo b >> @WD@/o", "echo b2 >> @WD@/o"]
    }
  }
}
EOF
  up; rc0 "up"; sleep 1; info "cross-pane command order" "$(paste -sd, "$WD/o")"; end

  begin cmd_path_override_env
  fx <<'EOF'
session {
  name = "po"
  envs = {
    PATH = "/nonexistent"
  }
  window {
    pane {
      commands = ["echo a", "echo b"]
    }
  }
}
EOF
  TO=8 up; rc0 "envs.PATH without tmux, then two commands"; end

  begin session_commands
  mkdir -p d1 d2
  fx <<'EOF'
session {
  name = "sc"
  commands = ["sleep 1; echo s1 > @WD@/o", "pwd >> @WD@/o"]
  window {
    name = "w1"
    starting_directory = "@WD@/d1"
    pane {
      starting_directory = "@WD@/d1"
    }
  }
  window {
    name = "w2"
    focus = true
    pane {
      starting_directory = "@WD@/d2"
    }
  }
}
EOF
  up; rc0 "up"; wf "$WD/o"; sleep 0.5
  eq "session commands ran in order in the active pane" "s1,$WD/d2" "$(paste -sd, "$WD/o" 2>/dev/null)"
  end

  local sname
  for sname in "my sess" "semi;colon" "quote'd"; do
    begin session_commands_name
    fx <<EOF
session {
  name = "$sname"
  commands = ["echo a > @WD@/o", "echo b >> @WD@/o"]
  window {
    pane {}
  }
}
EOF
    TO=10 up; rc0 "session commands with session name [$sname]"
    wf "$WD/o" 3; sleep 0.3; eq "session commands ran [$sname]" "a,b" "$(paste -sd, "$WD/o" 2>/dev/null)"
    end
  done

  begin pane_commands_many_windows_serial
  fx <<'EOF'
session {
  name = "mw"
  window {
    name = "w1"
    pane {
      commands = ["echo 1 >> @WD@/o", "echo 2 >> @WD@/o"]
    }
  }
  window {
    name = "w2"
    pane {
      commands = ["echo 3 >> @WD@/o", "echo 4 >> @WD@/o"]
    }
  }
}
EOF
  up; rc0 "up"; sleep 0.5; match "all commands ran" '1.*2.*3' "$(paste -sd, "$WD/o")"; end
}

t_options_hooks() {
  begin options_all_scopes
  fx <<'EOF'
session {
  name = "op"
  options = {
    "history-limit" = "4242"
    "status" = "off"
  }
  window {
    name = "w"
    options = {
      "automatic-rename" = "off"
      "monitor-activity" = "on"
    }
    pane {
      name = "p"
      options = {
        "remain-on-exit" = "on"
      }
    }
  }
}
EOF
  up; rc0 "up"
  eq "session option history-limit" "4242" "$(tm show -t op -v history-limit)"
  eq "session option status" "off" "$(tm show -t op -v status)"
  eq "window option automatic-rename" "off" "$(tm show -w -t op:w -v automatic-rename)"
  eq "window option monitor-activity" "on" "$(tm show -w -t op:w -v monitor-activity)"
  eq "pane option remain-on-exit" "on" "$(tm show -p -t op:w.0 -v remain-on-exit)"
  eq "window name survives automatic-rename" "w" "$(wnames op)"
  end

  begin options_session_declares_window_option
  # A window option declared on the session applies to every window, not only to the first.
  fx <<'EOF'
session {
  name = "ow"
  options = {
    "remain-on-exit" = "on"
    "history-limit"  = "4242"
  }
  window {
    name = "one"
    pane {}
  }
  window {
    name = "two"
    pane {}
  }
}
EOF
  up; rc0 "up"
  eq "window one remain-on-exit" "on" "$(tm show -w -t "$(wid ow one)" -v remain-on-exit)"
  eq "window two remain-on-exit" "on" "$(tm show -w -t "$(wid ow two)" -v remain-on-exit)"
  eq "session option history-limit" "4242" "$(tm show -t ow -v history-limit)"
  end

  begin options_window_declares_session_option
  # A session option declared on a window applies to the session, with a warning.
  fx <<'EOF'
session {
  name = "os"
  window {
    name    = "w"
    options = { "history-limit" = "4321" }
    pane {}
  }
}
EOF
  up; rc0 "up"
  eq "session option history-limit" "4321" "$(tm show -t os -v history-limit)"
  match "warns that the option applies to the session" "applies to the whole session" "$OUT$ERR"
  end

  begin options_numeric_value
  fx <<'EOF'
session {
  name = "on"
  options = {
    "history-limit" = 5000
  }
  window {
    pane {}
  }
}
EOF
  up; info "numeric (unquoted) option value" "rc=$RC val=$(tm show -t on -v history-limit) err=[$ERR]"; end

  begin options_invalid_name
  fx <<'EOF'
session {
  name = "oi"
  options = {
    "no-such-option" = "1"
  }
  window {
    pane {}
  }
}
EOF
  up; rcnz "unknown option name fails"
  info "unknown option aftermath" "session_left=$(has oi && echo yes || echo no) err=[$ERR]"; end

  begin options_invalid_value
  fx <<'EOF'
session {
  name = "ov"
  window {
    options = {
      "automatic-rename" = "maybe"
    }
    pane {}
  }
}
EOF
  up; rcnz "invalid option value fails"
  info "invalid option value aftermath" "session_left=$(has ov && echo yes || echo no) windows=$(wnames ov) err=[$ERR]"; end

  begin options_user_option
  fx <<'EOF'
session {
  name = "ou"
  options = {
    "@my-flag" = "yes"
  }
  window {
    pane {}
  }
}
EOF
  up; rc0 "user @option"; eq "user @option set" "yes" "$(tm show -t ou -v @my-flag)"; end

  begin hooks_all_scopes
  fx <<'EOF'
session {
  name = "hk"
  hooks = {
    "session-renamed" = "run-shell 'touch @WD@/h_session'"
  }
  window {
    name = "w"
    hooks = {
      "window-renamed" = "run-shell 'touch @WD@/h_window'"
    }
    pane {
      name = "p"
      hooks = {
        "pane-focus-in" = "run-shell 'touch @WD@/h_pane'"
      }
    }
  }
}
EOF
  up; rc0 "up"
  match "session hook registered" 'session-renamed' "$(tm show-hooks -t hk)"
  match "window hook registered" 'window-renamed' "$(tm show-hooks -w -t hk:w)"
  match "pane hook registered" 'pane-focus-in' "$(tm show-hooks -p -t hk:w.0)"
  tm rename-window -t hk:w w2; we "$WD/h_window" 3 && ok "window hook fires" || ko "window hook fires" "no marker"
  tm rename-session -t hk hk2; we "$WD/h_session" 3 && ok "session hook fires" || ko "session hook fires" "no marker"
  end

  begin hooks_invalid
  fx <<'EOF'
session {
  name = "hi"
  hooks = {
    "no-such-hook" = "display hi"
  }
  window {
    pane {}
  }
}
EOF
  up; rcnz "unknown hook name fails"; info "unknown hook aftermath" "session_left=$(has hi && echo yes || echo no) err=[$ERR]"; end

  begin hooks_session_created
  fx <<'EOF'
session {
  name = "hc"
  hooks = {
    "session-created" = "run-shell 'touch @WD@/h_created'"
  }
  window {
    pane {}
  }
}
EOF
  up; rc0 "up"; sleep 1
  if [[ -e $WD/h_created ]]; then ok "session-created hook (README example) fires"; else ko "session-created hook (README example) fires" "hook is set after new-session, so it can never fire for this session"; fi
  end
}

t_envs() {
  begin envs_debug_redacted
  fx <<'EOF'
session {
  name = "er"
  envs = {
    TOKEN = "hunter2"
  }
  window {
    name = "w"
    pane {}
  }
}
EOF
  gz up --detached --debug --socket-name "$SOCK"; rc0 "up --debug with envs"
  nomatch "--debug does not print an env value" 'hunter2' "$OUT$ERR"
  match "--debug shows the env key with the value redacted" 'TOKEN <redacted>' "$OUT$ERR"
  eq "the session still gets the env value" "TOKEN=hunter2" "$(tm showenv -t er TOKEN)"
  end

  begin cmds_text_at_debug
  fx <<'EOF'
session {
  name = "ct"
  window {
    name = "w"
    pane {
      commands = ["true hunter3"]
    }
  }
}
EOF
  up; rc0 "up with a command"
  nomatch "the default log level does not print command text" 'hunter3' "$OUT$ERR"
  match "the default log level counts the commands" 'running pane commands.*count=1' "$OUT$ERR"
  tm kill-server
  gz up --detached --debug --socket-name "$SOCK"; rc0 "up --debug with a command"
  match "--debug prints command text" 'hunter3' "$OUT$ERR"
  end

  begin envs_session
  fx <<'EOF'
session {
  name = "ev"
  envs = {
    FOO = "bar baz"
    EMPTY = ""
    QUOTE = "it's \"q\""
  }
  window {
    name = "w1"
    pane {
      commands = ["printf '[%s][%s][%s]\\n' \"$FOO\" \"$EMPTY\" \"$QUOTE\" > @WD@/o1"]
    }
  }
  window {
    name = "w2"
    pane {
      commands = ["echo \"$FOO\" > @WD@/o2"]
    }
  }
}
EOF
  up; rc0 "up"; wf "$WD/o1"; wf "$WD/o2"
  eq "envs reach first window panes" "[bar baz][][it's \"q\"]" "$(cat "$WD/o1" 2>/dev/null)"
  eq "envs reach later windows" "bar baz" "$(cat "$WD/o2" 2>/dev/null)"
  eq "session environment has FOO" "FOO=bar baz" "$(tm showenv -t ev FOO)"
  end

  local scope
  for scope in window pane; do
    begin "envs_on_$scope"
    if [[ $scope == window ]]; then
      fx <<'EOF'
session {
  name = "ew"
  window {
    envs = {
      FOO = "x"
    }
    pane {}
  }
}
EOF
    else
      fx <<'EOF'
session {
  name = "ew"
  window {
    pane {
      envs = {
        FOO = "x"
      }
    }
  }
}
EOF
    fi
    up; rcnz "envs on $scope rejected"; no_server "envs on $scope"
    match "envs on $scope diagnostic points at session" 'session' "$ERR$OUT"
    info "envs on $scope diagnostic" "$ERR$OUT"
    end
  done
}
