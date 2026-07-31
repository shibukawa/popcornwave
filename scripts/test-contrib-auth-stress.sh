#!/bin/sh

# Repeat concurrency-sensitive authentication tests across compiler/runtime runs.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

go_bin=${GO:-go}
tinygo_bin=${TINYGO:-tinygo}
count=${COUNT:-10}
race_count=${RACE_COUNT:-5}
export GOCACHE=${GOCACHE:-/tmp/petitweb-go-stress-cache}

case "$count" in
	''|*[!0-9]*) echo "COUNT must be a positive integer" >&2; exit 2 ;;
esac
case "$race_count" in
	''|*[!0-9]*) echo "RACE_COUNT must be a positive integer" >&2; exit 2 ;;
esac

if [ "$count" -lt 1 ] || [ "$race_count" -lt 1 ]; then
	echo "COUNT and RACE_COUNT must be positive" >&2
	exit 2
fi
if ! command -v "$go_bin" >/dev/null 2>&1; then
	echo "missing Go executable: $go_bin" >&2
	exit 2
fi
if ! command -v "$tinygo_bin" >/dev/null 2>&1; then
	echo "missing TinyGo executable: $tinygo_bin" >&2
	exit 2
fi

echo "== host $($go_bin version) =="
echo "== tinygo $($tinygo_bin version) =="
echo "== TinyGo Passkey stress ($count runs) =="
"$tinygo_bin" test -count="$count" ./contrib/passkey
echo "== TinyGo OIDC stress ($count runs) =="
"$tinygo_bin" test -count="$count" ./contrib/oidc
echo "== Host race state/Passkey stress ($race_count runs) =="
"$go_bin" test -race -count="$race_count" ./authstate/memory ./contrib/passkey
