#!/usr/bin/env bash
# Shared helpers for the glaze release-binary E2E sweep.
set -u
export TERM=xterm-256color
G=${G:-/opt/glaze/glaze}
LABEL=${LABEL:-$(tmux -V | tr ' ' '_')}
OUT_DIR=/work/results/$LABEL
ROOT=/tmp/gz
N=0
REAL_HOME=$HOME

init_results() {
  rm -rf "$OUT_DIR"; mkdir -p "$OUT_DIR/logs"
  RES=$OUT_DIR/results.tsv; : >"$RES"
  { echo "label=$LABEL"; tmux -V; "$G" --version; uname -a; } >"$OUT_DIR/env.txt"
}

begin() {
  CUR=$1; N=$((N + 1)); SOCK=gz$N; WD=$ROOT/$N
  rm -rf "$WD"; mkdir -p "$WD/home"; export HOME=$WD/home; unset GLAZE_PATH
  cd "$WD" || exit 1
  LOG=$OUT_DIR/logs/$(printf '%03d' $N)-$CUR.log; : >"$LOG"
}

end() {
  tmux -L "$SOCK" kill-server 2>/dev/null
  pkill -f "script -qfec" 2>/dev/null
  cd /
}

# r CMD... runs a command under a timeout and records RC, OUT, ERR and DUR (ms).
r() {
  local to=${TO:-20} t0 t1
  t0=$(date +%s%N)
  timeout "$to" "$@" >"$WD/.out" 2>"$WD/.err" </dev/null
  RC=$?
  t1=$(date +%s%N); DUR=$(((t1 - t0) / 1000000))
  OUT=$(cat "$WD/.out"); ERR=$(cat "$WD/.err")
  { echo "\$ $*"; echo "rc=$RC dur=${DUR}ms"; echo "--stdout--"; echo "$OUT"; echo "--stderr--"; echo "$ERR"; echo; } >>"$LOG"
}
gz() { r "$G" "$@"; }
up() { gz up --detached --socket-name "$SOCK" "$@"; }
down() { gz down --socket-name "$SOCK" "$@"; }
tm() { tmux -L "$SOCK" "$@" 2>/dev/null; }

# fx FILE writes stdin to FILE (default .glaze) with @WD@ and @HOME@ substituted.
fx() { mkdir -p "$(dirname "${1:-.glaze}")"; sed -e "s|@WD@|$WD|g" -e "s|@HOME@|$HOME|g" >"${1:-.glaze}"; }

simple() {
  fx "${2:-.glaze}" <<EOF
session {
  name = "$1"
  window {
    name = "w"
    pane {
      name = "p"
    }
  }
}
EOF
}

rec() {
  local a=${1//$'\t'/\\t} d=${3:-}
  d=${d//$'\t'/\\t}
  printf '%s\t%s\t%s\t%s\n' "$CUR" "$a" "$2" "$d" | tr -d '\r' >>"$RES"
}
ok() { rec "$1" PASS; }
ko() {
  local d=${2//$'\n'/⏎}
  rec "$1" FAIL "${d:0:700}"
  echo "FAIL [$CUR] $1 :: ${d:0:300}"
}
info() { local d=${2//$'\n'/⏎}; rec "$1" INFO "${d:0:700}"; }
eq() { if [[ "$2" == "$3" ]]; then ok "$1"; else ko "$1" "expected [$2] got [$3]"; fi; }
match() { if [[ "$3" =~ $2 ]]; then ok "$1"; else ko "$1" "expected /$2/ got [$3]"; fi; }
nomatch() { if [[ "$3" =~ $2 ]]; then ko "$1" "unexpected /$2/ in [$3]"; else ok "$1"; fi; }
rc0() { if [[ $RC -eq 0 ]]; then ok "$1 rc=0"; else ko "$1 rc=0" "rc=$RC stderr=[$ERR] stdout=[${OUT:0:200}]"; fi; }
rcnz() {
  if [[ $RC -ne 0 && $RC -ne 124 ]]; then ok "$1 rc!=0"
  elif [[ $RC -eq 124 ]]; then ko "$1 rc!=0" "timed out (hang) after ${DUR}ms"
  else ko "$1 rc!=0" "rc=0 stdout=[${OUT:0:200}] stderr=[${ERR:0:300}]"; fi
}
no_server() { if tm ls >/dev/null; then ko "$1 no tmux server started" "server running: $(tm ls)"; else ok "$1 no tmux server started"; fi; }
has() { tm has-session -t "=$1"; }
exists() { if has "$2"; then ok "$1"; else ko "$1" "session [$2] missing; ls=[$(tm ls)]"; fi; }
gone() { if has "$2"; then ko "$1" "session [$2] still present"; else ok "$1"; fi; }

# Introspection. All targets use session ids or exact matches.
wins() { tm lsw -t "=$1" -F '#{window_index}|#{window_name}|#{window_active}|#{window_panes}'; }
# tmux 3.4 prints a $ that starts a variable name as \$ in plain -F output.
unesc34() { if [[ $(tmux -V) == "tmux 3.4" ]]; then sed 's/\\\$/$/g'; else cat; fi; }
wnames() { tm lsw -t "=$1" -F '#{window_name}' | unesc34 | paste -sd, -; }
wid() { tm lsw -t "=$1" -F '#{window_id}|#{window_name}' | awk -v n="$2" '{i=index($0,"|"); if (substr($0,i+1)==n) {print substr($0,1,i-1); exit}}'; }
ptitles() { tm lsp -t "$1" -F '#{pane_title}' | unesc34 | paste -sd, -; }
ppaths() { tm lsp -t "$1" -F '#{pane_current_path}' | paste -sd, -; }
pactive() { tm lsp -t "$1" -F '#{pane_active}#{pane_title}' | awk '/^1/{print substr($0,2)}'; }
wactive() { tm lsw -t "=$1" -F '#{window_active}#{window_name}' | awk '/^1/{print substr($0,2)}'; }
geo() { tm lsp -t "$1" -F '#{pane_left},#{pane_top},#{pane_width}x#{pane_height}' | paste -sd' ' -; }
snap() {
  local s=$1 w
  tm lsw -t "=$s" -F 'W #{window_index}|#{window_name}|#{window_active}|#{window_panes}'
  for w in $(tm lsw -t "=$s" -F '#{window_id}'); do
    tm lsp -t "$w" -F '  P #{pane_index}|#{pane_title}|#{pane_active}|#{pane_current_path}|#{pane_left},#{pane_top},#{pane_width}x#{pane_height}'
  done
}

# refgeo PRESET N builds a reference window with N panes on the same server and
# prints the geometry tmux itself produces for PRESET.
refgeo() {
  local s=ref$RANDOM i
  tm new-session -d -s "$s"
  for ((i = 1; i < $2; i++)); do tm splitw -t "=$s:"; tm select-layout -t "=$s:" tiled; done
  tm select-layout -t "=$s:" "$1"
  geo "=$s:"
  tm kill-session -t "=$s"
}

# wf FILE [SECS] waits for FILE to exist and be non-empty.
# wrc FILE waits up to 10 s for the RC= line that a command typed into a pane appends to FILE.
wrc() { local i; for ((i = 0; i < 100; i++)); do grep -q '^RC=' "$1" 2>/dev/null && return 0; sleep 0.1; done; return 1; }

wf() {
  local i
  for ((i = 0; i < ${2:-8} * 10; i++)); do [[ -s $1 ]] && return 0; sleep 0.1; done
  return 1
}
# we FILE [SECS] waits for FILE to exist.
we() {
  local i
  for ((i = 0; i < ${2:-8} * 10; i++)); do [[ -e $1 ]] && return 0; sleep 0.1; done
  return 1
}
