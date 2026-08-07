#!/usr/bin/env bash
# Builds both todo services in every configuration the comparison reports and
# prints one row per build.
#
# A configuration that does not compile is reported rather than skipped: for
# this pair of applications that is a result, not a gap. Run from this
# directory.
set -uo pipefail

cd "$(dirname "$0")"
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT

# size prints the byte count of $1, or a dash when the build left nothing.
size() { [ -f "$1" ] && stat -f%z "$1" 2>/dev/null || stat -c%s "$1" 2>/dev/null || echo "—"; }

# build runs one configuration and prints its size, or "fails" with the reason
# recorded in $out/<tag>.err for the caller to read.
build() {
  local tag=$1 dir=$2 pkg=$3
  shift 3
  local bin="$out/$tag"
  if (cd "$dir" && "$@" -o "$bin" "$pkg") >"$out/$tag.err" 2>&1; then
    size "$bin"
  else
    echo "fails"
  fi
}

row() {
  local label=$1 std=$2 pw=$3
  printf '%-28s %14s %14s\n' "$label" "$std" "$pw"
}

printf '%-28s %14s %14s\n' 'BUILD' 'net/http+pgx' 'Popcorn Wave'
row 'host go' \
  "$(build std-go stdhttp . go build)" \
  "$(build pw-go popcornwave ./cmd/popcornwave go build)"
row 'host go -ldflags="-s -w"' \
  "$(build std-go-s stdhttp . go build -ldflags=-s\ -w)" \
  "$(build pw-go-s popcornwave ./cmd/popcornwave go build -ldflags=-s\ -w)"
row 'tinygo native' \
  "$(build std-tg stdhttp . tinygo build)" \
  "$(build pw-tg popcornwave ./cmd/popcornwave tinygo build)"
row 'tinygo native -no-debug' \
  "$(build std-tgn stdhttp . tinygo build -no-debug)" \
  "$(build pw-tgn popcornwave ./cmd/popcornwave tinygo build -no-debug)"
row 'tinygo wasip1' \
  "$(build std-wa stdhttp . tinygo build -target=wasip1)" \
  "$(build pw-wa popcornwave ./cmd/popcornwave tinygo build -target=wasip1)"
row 'tinygo wasip1 -no-debug' \
  "$(build std-wan stdhttp . tinygo build -target=wasip1 -no-debug)" \
  "$(build pw-wan popcornwave ./cmd/popcornwave tinygo build -target=wasip1 -no-debug)"

echo
echo "First error line of each failing build:"
for f in "$out"/*.err; do
  [ -s "$f" ] || continue
  tag=$(basename "$f" .err)
  [ -f "$out/$tag" ] && continue
  printf '  %-8s %s\n' "$tag" "$(grep -m1 -E 'undefined|not found|error|cannot' "$f" | cut -c1-140)"
done
