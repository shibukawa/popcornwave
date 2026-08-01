---
title: Health and Readiness
description: The liveness, readiness, and OpenAPI endpoints the primary listener can expose, and why none of them has a default path.
sidebar:
  order: 1
---

An orchestrator needs two different questions answered. *Is this process alive,
or should it be killed and replaced?* And *can this process take traffic right
now?* They have different answers during a rolling deploy, which is why they are
two endpoints:

```toml
[server]
health = "/healthz"
readiness = "/readyz"
```

Neither key has a default. An application that answers on `/healthz` should say
so where an operator reading its configuration will see it — a default would
leave endpoints running that no file in the repository mentions. An unset key
registers no route at all.

`pw init` writes both into `config.dev.toml`, so a scaffolded project has them
in development and states them explicitly for every other environment.

## What each one answers

**`health` is liveness.** It checks nothing and depends on nothing. If the
process can accept a connection and run a handler, it answers `200`. That is the
correct definition: a dependency outage is not a reason to kill and restart your
application, and a liveness probe that fails when the database is down turns one
outage into a restart loop.

**`readiness` pings every configured database pool** — each connection in the
group set, or the single pool — with a one-second bound over the whole check.
Any pool that does not answer makes the endpoint `503`, and the orchestrator
stops routing to this instance until it recovers.

Both respond identically otherwise:

| | |
| --- | --- |
| Methods | `GET` and `HEAD`; anything else is `405` with `Allow` |
| Success | `200`, body `ok` |
| Failure | `503`, body `unavailable` |
| Content-Type | `text/plain; charset=utf-8` |
| Cache-Control | `no-store` |

The body is two words on purpose. These endpoints bypass session and
authentication, so they are reachable by anything that can reach the port, and
they reveal only status — never a DSN, a backend name, a stack trace, or a
configuration value. A probe that leaks the shape of your infrastructure to an
unauthenticated caller is a worse problem than the one it was added to detect.

## The OpenAPI endpoint

```toml
[server]
openapi = "/openapi.json"
api_doc = "scalar"
api_doc_path = "/docs"
```

The same listener can serve the generated OpenAPI document and a UI over it.
Both follow the same rule as the probes: no default path, and an unset key
serves nothing. `api_doc` additionally requires `openapi` — a UI over a document
nobody serves has nothing to render.

Where they differ is access. The probes are answered above every extension,
which is what makes them unauthenticated by construction. The documentation
endpoints sit *below* the extension chain, so the session and the authentication
guard reach them exactly as they reach an application route:

```toml
[auth.protection]
include = ["/openapi.json", "/docs"]
```

Protection is opt-in, so an unlisted path stays public — but a test that
authenticates can now read the document the same way it reads its own routes,
and a closed deployment can put its API description behind a login without
moving it off the primary listener.

Setting it per environment is still worth doing. `pw init` writes
`api_doc = "scalar"` into `config.dev.toml` only, and the default is empty,
which keeps the reference private until a staging or production config
deliberately opts in. See [API Documentation](/productivity/api-documentation/),
and [`pw doctor`](/pw/project/doctor/), which reports an exposed documentation
endpoint as a readiness finding for a deployed environment.

## Collisions fail startup

Every operational path is validated against the application's routes before the
listener opens. A route that collides with an enabled endpoint is a startup
error, not a silent shadowing — you find out at deploy time rather than from a
probe that has been reporting on the wrong handler.

## Shutting down

On `SIGINT` or `SIGTERM` the server stops accepting new connections and lets
in-flight requests finish, bounded by:

```toml
[server]
shutdown_timeout = "10s"
```

Runtime resources — database pools and anything else registered for cleanup —
are closed after that, under the same bound.

Requests still in flight when the timeout expires are cut off. Size the value
against your slowest ordinary request, and remember that a
[stream](/guides/frontend/streams/) is designed to stay open far longer than
that: a long-lived stream will be terminated by shutdown, and the client is
expected to reconnect.

The readiness endpoint does not start failing when shutdown begins. Draining is
the orchestrator's job — it stops sending traffic, then sends the signal — and
by the time the signal arrives, the listener is already closing.
