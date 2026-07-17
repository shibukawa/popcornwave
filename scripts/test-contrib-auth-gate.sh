#!/bin/sh

# Run the complete bounded authentication verification gate.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

script_dir=$ROOT/scripts
go_bin=${GO:-go}
tinygo_bin=${TINYGO:-tinygo}
export GOCACHE=${GOCACHE:-/tmp/petitweb-go-gate-cache}

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

echo '== evidence reference check =='
"$script_dir/check-contrib-auth-evidence.sh"

echo '== host/TinyGo package matrix =='
GO="$go_bin" TINYGO="$tinygo_bin" \
	"$script_dir/test-contrib-auth.sh"

echo '== focused OAuth/OIDC interoperability =='
GO="$go_bin" TINYGO="$tinygo_bin" \
	"$script_dir/test-contrib-auth-interop.sh"

echo '== go vet =='
"$go_bin" vet ./contrib/...

echo '== race gate =='
GO="$go_bin" "$script_dir/test-contrib-auth-race.sh"

echo '== fuzz smoke gate =='
FUZZTIME=${FUZZTIME:-1s} GO="$go_bin" \
	"$script_dir/fuzz-contrib-auth.sh"
