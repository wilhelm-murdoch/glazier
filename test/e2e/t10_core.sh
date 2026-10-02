# CLI, profile resolution, structure, base index, directories, layouts, focus.

t_cli_basics() {
  begin cli_basics
  # matrix.sh tests a release and passes its version. A local build has no release version, so only the form is checked.
  gz --version; rc0 "--version"
  if [[ -n ${EXPECT_VERSION:-} ]]; then
    match "--version reports $EXPECT_VERSION" "Version: ${EXPECT_VERSION//./\\.}," "$OUT"
  else
    match "--version reports a version, a commit and a date" '^Version: [^,]+, Stage: [^,]+, Commit: [^,]+, Date: .+$' "$OUT"
  fi
  # With a profile in place, a -v that up ignores would build the session.
  simple vf; gz up -v --detached --socket-name "$SOCK"
  eq "-v after a subcommand is a usage error (exit 2)" 2 "$RC"; no_server "-v after a subcommand"; rm -f .glaze
  gz --help; rc0 "--help"
  for c in up down ls format save; do gz "$c" --help; rc0 "$c --help"; done
  gz bogus; rcnz "unknown subcommand"
  gz up --no-such-flag; rcnz "unknown flag"
  simple lv
  gz --log-level nope up --detached --socket-name "$SOCK"; rcnz "invalid --log-level"
  for l in trace debug info warning error critical; do
    gz --log-level "$l" up --detached --clear --socket-name "$SOCK"
    rc0 "--log-level $l accepted"
  done
  tm kill-server
  gz --log-level error up --detached --socket-name "$SOCK"
  nomatch "--log-level error hides INF lines" 'INF' "$OUT$ERR"
  tm kill-server
  gz up --detached --socket-name "$SOCK"
  if [[ -n $OUT && -z $ERR ]]; then info "up logs go to stdout" "stdout has logs, stderr empty"; fi
  if [[ -t 1 ]]; then :; else nomatch "no ANSI escapes when output is not a TTY" $'\e\\[' "$OUT$ERR"; fi
  tm kill-server
  gz up --detached --debug --socket-name "$SOCK"
  rc0 "up --debug"
  match "--debug prints tmux commands" 'new-session|new|neww|splitw|split-window' "$OUT$ERR"
  end
}

t_resolution() {
  begin res_cwd
  simple rcwd; up; rc0 "profile from cwd"; exists "cwd session created" rcwd; end

  begin res_flag
  simple rflag x/prof.hcl; up --profile-path x/prof.hcl; rc0 "--profile-path non-.glaze name"; exists "flag session created" rflag; end

  begin res_glaze_path
  simple rgp gp/.glaze; mkdir -p empty; cd empty
  GLAZE_PATH=$WD/gp gz up --detached --socket-name "$SOCK"; rc0 "\$GLAZE_PATH fallback"; exists "GLAZE_PATH session" rgp; end

  begin res_glaze_path_tilde
  simple rgpt "$HOME/gp/.glaze"; mkdir -p empty; cd empty
  GLAZE_PATH='~/gp' gz up --detached --socket-name "$SOCK"; rc0 "\$GLAZE_PATH with ~"; exists "GLAZE_PATH ~ session" rgpt; end

  begin res_precedence
  simple rcwdwins; simple rgploses gp/.glaze
  GLAZE_PATH=$WD/gp gz up --detached --socket-name "$SOCK"
  exists "cwd beats GLAZE_PATH" rcwdwins; gone "GLAZE_PATH not used when cwd has .glaze" rgploses; end

  begin res_flag_beats_cwd
  simple rcwd2; simple rflag2 other.glaze; up --profile-path other.glaze
  exists "--profile-path beats cwd" rflag2; gone "cwd ignored with --profile-path" rcwd2; end

  begin res_tilde
  simple rtilde "$HOME/p.glaze"; up --profile-path '~/p.glaze'; rc0 "--profile-path with ~"; exists "tilde session" rtilde; end

  begin res_missing
  mkdir -p empty; cd empty; up; rcnz "no profile anywhere"
  match "missing profile message mentions profile" '[Pp]rofile|glaze|[Nn]ot found|[Nn]o such' "$ERR$OUT"
  info "missing profile message" "$ERR$OUT"; no_server "missing profile"; end

  begin res_flag_missing
  up --profile-path nope.glaze; rcnz "--profile-path to missing file"; info "missing flag file message" "$ERR$OUT"; end

  begin res_dir
  mkdir -p adir; up --profile-path adir; rcnz "--profile-path to a directory"; info "dir profile message" "$ERR$OUT"; end
}

t_structure() {
  begin struct_basic
  fx <<'EOF'
session {
  name = "st"
  window {
    name = "w1"
    layout = "even-horizontal"
    pane {
      name = "a"
    }
    pane {
      name = "b"
    }
    pane {
      name = "c"
    }
  }
  window {
    name = "w2"
    pane {
      name = "d"
    }
  }
  window {
    name = "w3"
    layout = "even-vertical"
    pane {
      name = "e"
    }
    pane {
      name = "f"
    }
  }
}
EOF
  up; rc0 "up"
  eq "window order" "w1,w2,w3" "$(wnames st)"
  eq "window indexes start at base-index 0" "0,1,2" "$(tm lsw -t =st -F '#{window_index}' | paste -sd, -)"
  eq "w1 pane order" "a,b,c" "$(ptitles "$(wid st w1)")"
  eq "w1 pane indexes" "0,1,2" "$(tm lsp -t "$(wid st w1)" -F '#{pane_index}' | paste -sd, -)"
  eq "w2 panes" "d" "$(ptitles "$(wid st w2)")"
  eq "w3 pane order" "e,f" "$(ptitles "$(wid st w3)")"
  eq "one session only" "1" "$(tm ls | wc -l)"
  snap st >"$OUT_DIR/logs/struct_basic.snap"
  end

  begin struct_defaults
  fx <<'EOF'
session {
  window {
    pane {}
    pane {}
  }
}
EOF
  up; rc0 "up with no names"
  exists "session name defaults to 'default'" default
  eq "window name defaults to 'default'" "default" "$(wnames default)"
  eq "pane names default to 'default'" "default,default" "$(ptitles "=default:")"
  end
}

t_base_index() {
  local bi pbi
  for cfg in "1 1" "1 0" "0 1" "5 3"; do
    read -r bi pbi <<<"$cfg"
    begin "base_index_${bi}_${pbi}"
    printf 'set -g base-index %s\nset -g pane-base-index %s\n' "$bi" "$pbi" >"$HOME/.tmux.conf"
    fx <<'EOF'
session {
  name = "bi"
  window {
    name = "w1"
    pane {
      name = "a"
    }
    pane {
      name = "b"
    }
  }
  window {
    name = "w2"
    pane {
      name = "c"
    }
    pane {
      name = "d"
    }
    pane {
      name = "e"
    }
  }
}
EOF
    up; rc0 "up with base-index $bi pane-base-index $pbi"
    eq "windows (bi=$bi)" "w1,w2" "$(wnames bi)"
    eq "w1 panes (pbi=$pbi)" "a,b" "$(ptitles "$(wid bi w1)")"
    eq "w2 panes (pbi=$pbi)" "c,d,e" "$(ptitles "$(wid bi w2)")"
    eq "first window index" "$bi" "$(tm lsw -t =bi -F '#{window_index}' | head -1)"
    eq "first pane index" "$pbi" "$(tm lsp -t "$(wid bi w1)" -F '#{pane_index}' | head -1)"
    end
  done

  begin base_index_renumber
  printf 'set -g base-index 1\nset -g renumber-windows on\n' >"$HOME/.tmux.conf"
  fx <<'EOF'
session {
  name = "rn"
  window {
    name = "w1"
    pane {
      name = "a"
    }
  }
  window {
    name = "w2"
    pane {
      name = "b"
    }
  }
}
EOF
  up; rc0 "up with renumber-windows on"
  eq "windows with renumber-windows" "w1,w2" "$(wnames rn)"
  end

  begin base_index_late
  # A tmux.conf that sets base-index in the background changes it while glaze provisions the session.
  printf "run-shell -b 'tmux -L %s set -g base-index 1'\n" "$SOCK" >"$HOME/.tmux.conf"
  fx <<'EOF'
session {
  name = "late"
  window {
    name = "one"
    pane {}
    pane {}
    pane {}
  }
  window {
    name = "two"
    pane {}
    pane {}
  }
}
EOF
  up; rc0 "up with base-index set in the background"
  sleep 0.3; eq "both declared windows exist" "one,two" "$(wnames late)"
  end
}

t_directories() {
  begin dirs_levels
  mkdir -p d/s d/w d/p "d/with space"
  fx <<'EOF'
session {
  name = "dl"
  starting_directory = "@WD@/d/s"
  window {
    name = "w1"
    pane {
      name = "a"
    }
  }
  window {
    name = "w2"
    starting_directory = "@WD@/d/w"
    pane {
      name = "b"
    }
    pane {
      name = "c"
      starting_directory = "@WD@/d/p"
    }
  }
  window {
    name = "w3"
    starting_directory = "@WD@/d/with space"
    pane {
      name = "e"
    }
  }
}
EOF
  up; rc0 "up"
  eq "session path" "$WD/d/s" "$(tm display -p -t dl '#{session_path}')"
  eq "pane inherits session dir" "$WD/d/s" "$(ppaths "$(wid dl w1)")"
  eq "pane inherits window starting_directory" "$WD/d/w,$WD/d/p" "$(ppaths "$(wid dl w2)")"
  eq "window dir with a space inherited" "$WD/d/with space" "$(ppaths "$(wid dl w3)")"
  # An attached client opens a new window in the session path; a command-line client would use its own directory.
  # neww waits until the client is attached, and the client stays open until the window exists; both race under load.
  {
    for _ in {1..50}; do [[ -n $(tm lsc -t =dl) ]] && break; sleep 0.1; done
    printf 'neww -n later\n'
    for _ in {1..50}; do [[ -n $(wid dl later) ]] && break; sleep 0.1; done
  } | tm -C attach -t =dl >/dev/null
  eq "new window opened later by user uses session dir" "$WD/d/s" "$(ppaths "$(wid dl later)")"
  end

  begin dirs_default_cwd
  mkdir -p here; cd here; simple dc; up; rc0 "up"
  eq "no starting_directory uses cwd" "$WD/here" "$(ppaths =dc:)"
  end

  begin dirs_tilde
  mkdir -p "$HOME/proj"
  fx <<'EOF'
session {
  name = "dt"
  starting_directory = "~/proj"
  window {
    name = "proj"
    pane {}
  }
  window {
    name = "home"
    starting_directory = "~"
    pane {}
  }
}
EOF
  up; rc0 "up with ~ in starting_directory"; eq "~ expanded" "$HOME/proj" "$(ppaths "$(wid dt proj)")"
  eq "bare ~ expanded" "$HOME" "$(ppaths "$(wid dt home)")"
  end

  begin dirs_tilde_user
  fx <<'EOF'
session {
  name = "dtu"
  starting_directory = "~root/x"
  window {
    pane {}
  }
}
EOF
  up; rcnz "~user rejected"; no_server "~user"
  match "~user diagnostic says glaze expands only ~ and ~/" 'not `~user`' "$ERR"
  end

  begin dirs_relative
  mkdir -p prof/sub; mkdir -p elsewhere
  fx prof/.glaze <<'EOF'
session {
  name = "dr"
  starting_directory = "sub"
  window {
    pane {}
  }
}
EOF
  cd elsewhere; up --profile-path ../prof/.glaze
  rc0 "up with a relative starting_directory from another directory"
  eq "relative starting_directory is relative to the profile" "$WD/prof/sub" "$(ppaths =dr:)"
  end

  begin dirs_missing
  fx <<'EOF'
session {
  name = "dm"
  window {
    starting_directory = "/does/not/exist"
    pane {}
  }
}
EOF
  up; rcnz "missing starting_directory rejected"; no_server "missing dir"
  match "missing dir diagnostic" '[Dd]irector' "$ERR$OUT"
  end

  begin dirs_is_file
  touch afile
  fx <<'EOF'
session {
  name = "df"
  starting_directory = "@WD@/afile"
  window {
    pane {}
  }
}
EOF
  up; rcnz "starting_directory pointing at a file rejected"
  end
}

layout_fixture() { # NAME LAYOUT_LINE NPANES
  local i
  {
    echo 'session {'; echo "  name = \"$1\""; echo '  window {'; echo '    name = "w"'
    [[ -n $2 ]] && echo "    layout = \"$2\""
    for ((i = 0; i < $3; i++)); do echo '    pane {'; echo "      name = \"p$i\""; echo '    }'; done
    echo '  }'; echo '}'
  } >.glaze
}

t_layouts() {
  local p n
  for p in even-horizontal even-vertical main-horizontal main-vertical tiled; do
    for n in 3 4; do
      begin "layout_${p}_$n"
      layout_fixture lay "$p" "$n"; up; rc0 "up $p x$n"
      eq "$p x$n geometry matches tmux" "$(refgeo "$p" "$n")" "$(geo =lay:w)"
      eq "$p x$n pane order" "$(seq -s, -f 'p%g' 0 $((n - 1)))" "$(ptitles =lay:w)"
      end
    done
  done

  begin layout_default_tiled
  layout_fixture lay "" 4; up; rc0 "up"
  eq "omitted layout is tiled" "$(refgeo tiled 4)" "$(geo =lay:w)"
  end

  begin layout_raw_replay
  tm new-session -d -s ref; tm splitw -h -l 20 -t =ref:; tm splitw -v -l 5 -t =ref:
  raw=$(tm display -p -t ref: '#{window_layout}'); want=$(geo =ref:); tm kill-session -t =ref
  layout_fixture lay "$raw" 3; up; rc0 "up raw layout $raw"
  eq "raw layout geometry replayed" "$want" "$(geo =lay:w)"
  end

  begin layout_raw_bad_checksum
  tm new-session -d -s ref; tm splitw -h -t =ref:
  raw=$(tm display -p -t ref: '#{window_layout}'); tm kill-session -t =ref; tm kill-server
  bad="0000${raw:4}"; [[ ${raw:0:4} == 0000 ]] && bad="ffff${raw:4}"
  layout_fixture lay "$bad" 2; up; rcnz "raw layout with bad checksum fails at up"
  info "bad checksum aftermath" "rc=$RC session_left=$(has lay && echo yes || echo no) err=[$ERR]"
  end

  begin layout_raw_wrong_pane_count
  tm new-session -d -s ref; tm splitw -h -t =ref:; tm splitw -h -t =ref:
  raw=$(tm display -p -t ref: '#{window_layout}'); tm kill-session -t =ref; tm kill-server
  layout_fixture lay "$raw" 2; up
  info "wrong pane count aftermath" "rc=$RC session_left=$(has lay && echo yes || echo no) err=[$ERR]"
  end

  for p in "foo" "Tiled" "tiled " "rm -rf /" "bb62,80x24"; do
    begin layout_invalid
    layout_fixture lay "$p" 2; up; rcnz "invalid layout [$p] rejected"; no_server "invalid layout [$p]"
    end
  done
}

t_focus() {
  begin focus_window_first
  fx <<'EOF'
session {
  name = "fw"
  window {
    name = "w1"
    focus = true
    pane {}
  }
  window {
    name = "w2"
    pane {}
  }
  window {
    name = "w3"
    pane {}
  }
}
EOF
  up; rc0 "up"; eq "focused first window is active" "w1" "$(wactive fw)"; end

  begin focus_window_middle
  fx <<'EOF'
session {
  name = "fw"
  window {
    name = "w1"
    pane {}
  }
  window {
    name = "w2"
    focus = true
    pane {}
  }
  window {
    name = "w3"
    pane {}
  }
}
EOF
  up; rc0 "up"; eq "focused middle window is active" "w2" "$(wactive fw)"; end

  begin focus_window_none
  fx <<'EOF'
session {
  name = "fw"
  window {
    name = "w1"
    pane {}
  }
  window {
    name = "w2"
    pane {}
  }
}
EOF
  up; info "active window with no focus declared" "$(wactive fw)"; end

  local pos
  for pos in 0 1 2; do
    begin "focus_pane_$pos"
    {
      echo 'session {'; echo '  name = "fp"'; echo '  window {'; echo '    name = "w"'; echo '    layout = "even-horizontal"'
      for i in 0 1 2; do echo '    pane {'; echo "      name = \"p$i\""; [[ $i == "$pos" ]] && echo '      focus = true'; echo '    }'; done
      echo '  }'; echo '}'
    } >.glaze
    up; rc0 "up"; eq "focused pane p$pos is active" "p$pos" "$(pactive =fp:w)"
    end
  done

  begin focus_pane_second_window
  fx <<'EOF'
session {
  name = "fp"
  window {
    name = "w1"
    focus = true
    pane {
      name = "a"
    }
    pane {
      name = "b"
      focus = true
    }
    pane {
      name = "c"
    }
  }
  window {
    name = "w2"
    pane {
      name = "d"
      focus = true
    }
    pane {
      name = "e"
    }
  }
}
EOF
  up; rc0 "up"
  eq "w1 focused" "w1" "$(wactive fp)"; eq "w1 pane b focused" "b" "$(pactive =fp:w1)"; eq "w2 pane d focused" "d" "$(pactive =fp:w2)"
  end

  begin focus_pane_none
  layout_fixture fp tiled 3; up; info "active pane with no focus declared" "$(pactive =fp:w)"; end
}
