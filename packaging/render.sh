#!/usr/bin/env bash
# Prints a packaging template with @VERSION@ and each @SHA256_<asset>@ filled from a SHA256SUMS file.
# Usage: packaging/render.sh <tag> <SHA256SUMS> <template>
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <tag> <SHA256SUMS> <template>" >&2
  exit 2
fi

tag=$1
sums=$2
template=$3

# A pre-release tag such as v1.0.0-rc1 is not a valid version for pacman, so only plain releases get packages.
if [[ ! $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "render: $tag is not a vX.Y.Z release tag" >&2
  exit 1
fi

out=$(sed "s|@VERSION@|${tag#v}|g" "$template")

while read -r sum asset; do
  out=${out//@SHA256_${asset}@/$sum}
done < "$sums"

if grep -Eq '@(VERSION|SHA256_[^@]+)@' <<<"$out"; then
  echo "render: $template has a placeholder that $sums does not fill:" >&2
  grep -Eo '@(VERSION|SHA256_[^@]+)@' <<<"$out" | sort -u >&2
  exit 1
fi

printf '%s\n' "$out"
