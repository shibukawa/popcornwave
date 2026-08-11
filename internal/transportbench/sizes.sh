#!/usr/bin/env bash
# Builds one application for every target this framework supports and prints a
# row per build.
#
# The subject is examples/helloworld, which is a real application rather than a
# handler in a test: it binds configuration, opens a database, renders a page
# through the template compiler, and serves the browser runtime. A benchmark
# server would produce smaller and less useful numbers, because what a
# deployment ships is an application.
#
# A configuration that does not compile is reported rather than skipped. For
# this matrix that is a result: the fourth quadrant is a compile check, and
# whether it holds is the thing worth printing.
#
#   ./internal/transportbench/sizes.sh
set -uo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
app="$root/examples/helloworld"
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT

size() {
  [ -f "$1" ] || { echo "—"; return; }
  local bytes
  bytes=$(stat -f%z "$1" 2>/dev/null || stat -c%s "$1" 2>/dev/null)
  # MiB to one decimal, which is the resolution a reader compares at.
  awk -v b="$bytes" 'BEGIN { printf "%.1f MiB", b / 1048576 }'
}

build() {
  local tag=$1
  shift
  local bin="$out/$tag"
  if (cd "$app" && "$@" -o "$bin" ./cmd/helloworld) >"$out/$tag.err" 2>&1; then
    size "$bin"
  else
    echo "fails"
  fi
}

printf '%-34s %14s %14s\n' 'BUILD' 'net/http' 'fasthttp'
printf '%-34s %14s %14s\n' '----------------------------------' '--------------' '--------------'

row() { printf '%-34s %14s %14s\n' "$1" "$2" "$3"; }

row 'go build' \
  "$(build go-net go build)" \
  "$(build go-fast go build -tags fasthttp)"
row 'go build -ldflags="-s -w"' \
  "$(build go-net-s go build -ldflags=-s\ -w)" \
  "$(build go-fast-s go build -tags fasthttp -ldflags=-s\ -w)"
row 'tinygo build' \
  "$(build tg-net tinygo build)" \
  "$(build tg-fast tinygo build -tags fasthttp)"
row 'tinygo build -no-debug' \
  "$(build tg-net-n tinygo build -no-debug)" \
  "$(build tg-fast-n tinygo build -tags fasthttp -no-debug)"

echo
echo 'First error line of each failing build:'
printed=0
for f in "$out"/*.err; do
  [ -s "$f" ] || continue
  tag=$(basename "$f" .err)
  [ -f "$out/$tag" ] && continue
  printf '  %-12s %s\n' "$tag" "$(grep -m1 -E 'undefined|not found|error|cannot|unsupported' "$f" | cut -c1-160)"
  printed=1
done
[ "$printed" = 0 ] && echo '  (none)'
