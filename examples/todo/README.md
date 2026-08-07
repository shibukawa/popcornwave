# todo — the same application twice

One todo list, written two ways against one PostgreSQL table, so that binary
size and throughput can be compared without arguing about whether the two
programs do the same thing.

| Directory | What it is |
| --- | --- |
| `stdhttp/` | `net/http`, `html/template`, `encoding/json`, `pgx` |
| `popcornwave/` | Popcorn Wave: typed templates, typed SQL, generated binding |

Both serve the same five routes — an HTML list, a JSON list, and three form
posts — and `load.sh` refuses to measure them if their JSON bodies disagree on
the row count.

## Running one

```sh
docker run -d --name todo-pg \
  -e POSTGRES_USER=todo -e POSTGRES_PASSWORD=todo -e POSTGRES_DB=todo \
  -p 5433:5432 postgres:17-alpine
docker exec -i todo-pg psql -U todo -d todo < schema.sql
```

```sh
cd stdhttp && go run .              # http://127.0.0.1:8081
```

```sh
cd popcornwave && APP_ENV=dev go run ./cmd/popcornwave   # http://127.0.0.1:8082
```

## Comparing binary size

```sh
./sizes.sh
```

Six configurations per service. A build that fails is reported as `fails` with
its first error, because for this pair that is a result rather than a gap: `pgx`
does not compile under TinyGo at all, and neither service reaches `wasip1`.

## Comparing throughput

```sh
./load.sh
```

Needs [k6](https://k6.io). It seeds a fixed 50 rows, starts both services, and
alternates k6 between them so that a machine which gets busier partway through
moves both numbers rather than only the second.

Both are ordinary compiled binaries; the script does not run `pw dev`. For the
loopback HTTP measurement it copies `config.bench.toml` to a temporary
`config.dev.toml` and overrides only `SESSION_COOKIE_SECURE=false`. k6's
per-VU cookie jar can therefore keep one session across iterations, while the
logging, connection-pool, and middleware settings stay those of the benchmark.

`stdhttp/` can run on either database layer, which is how the framework's share
of the gap was separated from its database access:

```sh
DB_DRIVER=pgxpool DB_POOL=25 go run .   # pgx's own pool
DB_DRIVER=sqldb   DB_POOL=25 go run .   # database/sql + pgx/stdlib, as the framework does
```

Best of five alternating passes, all at 25 connections:

| | requests/s |
| --- | --- |
| `stdhttp` + pgxpool | 13,477 |
| `stdhttp` + `database/sql` + pgx/stdlib | 13,026 |

The database layer is worth 3 points. A CPU profile showing
`database/sql.withLock` at 15% of samples suggests it should be worth far more,
which is the point of running the control: a lock can hold CPU share without
being what limits throughput.

The larger lever is `session.backend` in `popcornwave/config.bench.toml`. Same
application code, five alternating passes each:

| | requests/s | HTML p95 |
| --- | --- | --- |
| `net/http` + pgx | ~14,700 | 3.15–3.39 ms |
| `popcornwave`, `cookie` | ~13,700 | 2.90–3.21 ms |
| `popcornwave`, `dev-volatile` | ~15,800 | 2.46–2.66 ms |

A cookie session sends 631 bytes of `Set-Cookie` per response against 299, and
the browser returns all of it on every request. The AES-GCM sealing measures
0.5 µs, so the cost is bytes rather than crypto. `dev-volatile` is a development
backend; `rdb`, `redis`, and `dynamo` are its production shapes and each adds a
storage round trip in exchange for the bytes.

Measured after PostgreSQL requests moved to the pgx-native pool, which was worth
about ten points end to end: the cookie row was 18% behind the baseline before
it and is 8% behind now, and `dev-volatile` went from level to 9% ahead. Per
request the framework spends 203 µs of CPU against the baseline's 221.

A session on a busier machine put the baseline at 11,400–11,700 rather than
~14,700 and left both ratios recognisable, which is why these tables are for the
ratios rather than the absolute rates.

`k6/load.js` sends `Accept: text/html` on the page request because the framework
issues a CSRF token only for a request that looks like a document. A generator
that omits it is not measuring the page path, and the form render fails.

The load is read-only. Writes would grow the table during the run, so whichever
service went second would be answering a larger query.

## What the comparison is not

The two services are not feature-equivalent, and the size and throughput numbers
should be read knowing which way that cuts.

`popcornwave/` additionally has configuration binding per environment, an
OpenAPI document, `/healthz` and `/readyz`, structured errors, security headers,
request IDs, a session, and a CSRF token in every form. `stdhttp/` has one
middleware that sets a request ID header. So the framework carries more bytes
and does more per request, and both show up in the measurements.

Making them equal would mean writing all of that by hand in `stdhttp/`, which is
the comparison the framework exists to avoid — so the honest presentation is to
measure what each actually is and say what the difference includes.

`config.bench.toml` is the configuration the load test uses. It exists because
`config.dev.toml` logs at debug and prints every statement the database runs,
which costs several times the work being measured.
