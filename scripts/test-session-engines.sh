#!/bin/sh

# Run the session and authentication-state stores against live PostgreSQL and
# MySQL servers. SQLite needs no server and runs in the ordinary test suite.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

postgres_name=petitweb-store-postgres-$$
mysql_name=petitweb-store-mysql-$$
postgres_port=${POSTGRES_TEST_PORT:-55432}
mysql_port=${MYSQL_TEST_PORT:-53306}

cleanup() {
	docker stop "$postgres_name" "$mysql_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

wait_ready() {
	name=$1
	shift
	attempt=0
	until docker exec "$name" "$@" >/dev/null 2>&1; do
		attempt=$((attempt + 1))
		if [ "$attempt" -ge 60 ]; then
			echo "timed out waiting for $name" >&2
			return 1
		fi
		sleep 1
	done
}

echo '== starting servers =='
docker run --rm -d --name "$postgres_name" -p "127.0.0.1:$postgres_port:5432" \
	-e POSTGRES_PASSWORD=pw -e POSTGRES_USER=pw -e POSTGRES_DB=pw \
	"${POSTGRES_TEST_IMAGE:-postgres:17-alpine}" >/dev/null
docker run --rm -d --name "$mysql_name" -p "127.0.0.1:$mysql_port:3306" \
	-e MYSQL_ROOT_PASSWORD=root -e MYSQL_USER=pw -e MYSQL_PASSWORD=pw -e MYSQL_DATABASE=pw \
	"${MYSQL_TEST_IMAGE:-mysql:8}" >/dev/null
wait_ready "$postgres_name" pg_isready -U pw
wait_ready "$mysql_name" mysqladmin ping -upw -ppw

PW_POSTGRES_TEST_DSN="postgres://pw:pw@127.0.0.1:$postgres_port/pw?sslmode=disable" \
	PW_MYSQL_TEST_DSN="mysql://pw:pw@tcp(127.0.0.1:$mysql_port)/pw" \
	go test ./sessionstore/ ./authstate/ ./database/ -count=1
