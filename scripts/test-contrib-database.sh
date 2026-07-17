#!/bin/sh

# Verify every supported SQLite backend. TinyGo is optional locally; CI should
# provide TINYGO or set TINYGO_DOCKER_IMAGE.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

go_bin=${GO:-go}
export GOCACHE=${GOCACHE:-/tmp/petitweb-go-database-cache}

if ! command -v "$go_bin" >/dev/null 2>&1; then
	echo "missing Go executable: $go_bin" >&2
	exit 2
fi

echo '== standard Go with cgo (mattn facade and native driver) =='
CGO_ENABLED=1 "$go_bin" test ./contrib/database/cgosqlite ./contrib/database/sqlite ./contrib/authstate/sqlite

echo '== standard Go without cgo (modernc facade) =='
CGO_ENABLED=0 "$go_bin" test ./contrib/database/cgosqlite ./contrib/database/sqlite ./contrib/authstate/sqlite

echo '== forced TinyGo logic under standard Go (native driver) =='
CGO_ENABLED=1 "$go_bin" test -tags force_tinygo_logic ./contrib/database/cgosqlite ./contrib/database/sqlite ./contrib/authstate/sqlite

if [ -n "${TINYGO_DOCKER_IMAGE:-}" ]; then
	echo "== TinyGo via $TINYGO_DOCKER_IMAGE =="
	docker run --rm -v "$ROOT:/src" -w /src \
		-e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod \
		"$TINYGO_DOCKER_IMAGE" tinygo test \
		./contrib/database/cgosqlite ./contrib/database/sqlite ./contrib/authstate/sqlite
elif command -v "${TINYGO:-tinygo}" >/dev/null 2>&1; then
	echo '== TinyGo native driver =='
	"${TINYGO:-tinygo}" test ./contrib/database/cgosqlite ./contrib/database/sqlite ./contrib/authstate/sqlite
else
	echo 'TinyGo test skipped: set TINYGO or TINYGO_DOCKER_IMAGE' >&2
fi
