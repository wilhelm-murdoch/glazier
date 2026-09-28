#!/usr/bin/env bash
# Builds one image per tmux target and runs the sweep in each image in parallel.
# Usage: ./matrix.sh [GLAZE_VERSION] [TARGET ...]. ONLY=regex limits the test groups.
set -u
cd "$(dirname "$0")" || exit 1

VERSION=${1:-v0.1.6}
shift 2>/dev/null
TARGETS=("$@")
((${#TARGETS[@]})) || TARGETS=(bookworm trixie jammy alpine)

base_for() {
  case $1 in
    bookworm) echo debian:bookworm-slim ;;
    trixie) echo debian:trixie-slim ;;
    jammy) echo ubuntu:22.04 ;;
    noble) echo ubuntu:24.04 ;;
    *) echo "unknown target: $1" >&2; exit 1 ;;
  esac
}

for t in "${TARGETS[@]}"; do
  if [[ $t == alpine ]]; then
    docker build -q -f Dockerfile.alpine --build-arg GLAZE_VERSION="$VERSION" -t "glaze-e2e:$t-$VERSION" . >/dev/null || exit 1
  else
    docker build -q --build-arg BASE="$(base_for "$t")" --build-arg GLAZE_VERSION="$VERSION" -t "glaze-e2e:$t-$VERSION" . >/dev/null || exit 1
  fi
done

mkdir -p results
for t in "${TARGETS[@]}"; do
  docker run --rm -v "$PWD":/work -e LABEL="$t" -e ONLY="${ONLY:-}" "glaze-e2e:$t-$VERSION" bash /work/run.sh >"results/$t.out" 2>&1 &
done
wait

status=0
for t in "${TARGETS[@]}"; do
  printf '%-10s %s  %s\n' "$t" "$(docker run --rm "glaze-e2e:$t-$VERSION" tmux -V)" "$(tail -1 "results/$t.out")"
  awk -F'\t' '$3=="FAIL"{print $1"|"$2}' "results/$t/results.tsv" | sort >"results/$t.fail"
  [[ -s results/$t.fail ]] && status=1
done

first=${TARGETS[0]}
for t in "${TARGETS[@]:1}"; do
  if ! diff -q "results/$first.fail" "results/$t.fail" >/dev/null; then
    echo "failure set differs between $first and $t:"
    diff "results/$first.fail" "results/$t.fail"
  fi
done
exit $status
