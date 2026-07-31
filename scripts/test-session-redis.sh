#!/bin/sh

# Run the session store against live Redis and Valkey containers.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

redis_name=petitweb-session-redis-$$
valkey_name=petitweb-session-valkey-$$
redis_port=${REDIS_TEST_PORT:-16381}
valkey_port=${VALKEY_TEST_PORT:-16382}

cleanup() {
	docker stop "$redis_name" "$valkey_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

wait_ready() {
	name=$1
	cli=$2
	attempt=0
	until docker exec "$name" "$cli" ping >/dev/null 2>&1; do
		attempt=$((attempt + 1))
		if [ "$attempt" -ge 30 ]; then
			echo "timed out waiting for $name" >&2
			return 1
		fi
		sleep 1
	done
}

echo '== Redis session store interoperability =='
docker run --rm -d --name "$redis_name" -p "127.0.0.1:$redis_port:6379" \
	"${REDIS_TEST_IMAGE:-redis:8.4-alpine}" >/dev/null
wait_ready "$redis_name" redis-cli
PETITWEB_REDIS_ADDR="127.0.0.1:$redis_port" \
	go test ./plugin/session/redis -run TestLiveRedisOrValkey -count=1
docker stop "$redis_name" >/dev/null

echo '== Valkey session store interoperability =='
docker run --rm -d --name "$valkey_name" -p "127.0.0.1:$valkey_port:6379" \
	"${VALKEY_TEST_IMAGE:-valkey/valkey:latest}" >/dev/null
wait_ready "$valkey_name" valkey-cli
PETITWEB_REDIS_ADDR="127.0.0.1:$valkey_port" \
	go test ./plugin/session/redis -run TestLiveRedisOrValkey -count=1
docker stop "$valkey_name" >/dev/null
