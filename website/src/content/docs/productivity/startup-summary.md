---
title: Configuration Summary
description: Check which configuration values actually took effect, and where each one came from, in one report per start.
sidebar:
  order: 5
---

Resolved configuration is reported **once**, not one record per key. What that
looks like depends on who is reading. On an interactive terminal the summary is
a tree, ending with the address the listener accepted:

```
   .-.   .-.
 .(   ) (   ).    Popcorn Wave v0.1.0
(   o     o   )   started at 2026-07-27 23:31:04 JST
(    \___/    )   env dev · config.dev.toml
 '-.__.___.__-'

configuration
├─ middleware
│  ├─ access_log       true
│  ├─ compression      true  ← file
│  └─ request_timeout  0s
├─ server
│  ├─ port          8080
│  └─ read_timeout  30s
└─ session
   └─ enabled  false

listening on http://localhost:8080
```

Only values that came from somewhere other than the built-in defaults are
marked: `← file`, `← env`, or `← flag`.

The last line is the address the listener accepted, which is not always the
`server.port` above it: a development run moves off a port it cannot bind, and
the summary keeps both — what was configured, and what answers. See
[`pw dev`](/pw/project/dev/#the-port).

Everywhere else — a pipe, a container, a log collector — the same facts become
one structured record instead, so a JSON handler or an OpenTelemetry bridge
ships a single event rather than sixty:

```json
{"time":"2026-07-27T23:31:04+09:00","level":"INFO","msg":"popcornwave started",
 "environment":"dev","config_file":"config.dev.toml",
 "listening":"http://localhost:8080",
 "config":{"server":{"port":"8080"},"session":{"enabled":"false"}},
 "config_source":{"middleware.compression":"file"}}
```

`observability.boot_log` overrides the choice:

| Value | Behavior |
| --- | --- |
| `auto` (default) | tree on a terminal, one record otherwise |
| `tree` | always the tree, written to stderr |
| `record` | always one record through the default `slog` logger |
| `off` | no startup summary |

When the application owns the listener — `pw.Middlewares` instead of `pw.Run` —
the summary is emitted after initialization, without the `listening` line.

## Restarts under `pw dev`

Once per process is once per rebuild in the developer loop, and reprinting forty
lines because you saved a template pushes whatever you were reading off the
screen. So [`pw dev`](/pw/project/dev/) reads the summary its application
printed and reports the next one against it. A restart that resolved to the same
configuration says so and stops:

```
reloaded
```

A restart that resolved to something else shows the rows that moved, in the
section they came from:

```
└─ html
   └─ bot_async_timeout  5s → 10s  ← file
```

A key that appeared or went away is marked `← added` or `← removed`, which is
what turning `html.bot_detection` off looks like: the settings it gates leave
the report entirely. A key whose value survived a change of layer reads
`← default → env`, because where a value came from is reported for the same
reason the value is. The listening address, the environment, the config file,
and the framework version are compared beside the keys, so a run that moved off
a port it could not bind is one line rather than silence.

The first summary of a session is printed whole — nothing has been read yet, so
there is no shorter answer that is still true. `record` and `off` are untouched:
a collector deduplicates, and a developer who turned the summary off did not ask
for a reload line in its place.

## Secrets in logs

Sensitive values are masked in both formats. Ordinary values become `*****`;
a DSN retains its public location while its credentials and query string are
removed. The exact name-matching rule and the explicit `secret` overrides are
documented under [what the startup summary shows](/reference/configuration-declaration/#what-the-startup-summary-shows).

See [Configuration](/guides/architecture/configuration/) for where these values come from,
and [Slow Query Diagnostics](/productivity/query-diagnostics/) for the same idea
applied to SQL.
