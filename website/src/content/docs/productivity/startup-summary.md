---
title: Startup Summary
description: See which configuration values actually took effect, and where each one came from, in one record per start.
sidebar:
  order: 4
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

## Secrets in logs

Keys containing `secret`, `password`, `token`, `credential`, `dsn`, or
`private_key` appear as `[REDACTED]` in both formats.

See [Configuration](/guides/configuration/) for where these values come from,
and [Query Diagnostics](/productivity/query-diagnostics/) for the same idea
applied to SQL.
