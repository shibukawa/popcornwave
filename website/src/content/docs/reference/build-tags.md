---
title: Build Tags
description: Every build tag defined across Popcorn Wave, tinygodriver and tinybind-go — what each one selects, who passes it, and which ones the toolchain sets for you.
sidebar:
  order: 3
---

Popcorn Wave leans on build tags harder than most Go projects, because one source
tree produces binaries that share almost no runtime: net/http and fasthttp, host
Go and TinyGo, a development build and a shipped one. A tag is how a file joins
one of those and not the others.

Three repositories define tags you may end up passing, and they compose in one
build. This page lists all of them.

Two things a table cannot say, so they are here:

**A tag excludes a whole file, never a call.** So a file holding a net/http
handler must hold nothing the fasthttp build needs — types, helpers and mux
wiring live in files of their own. This is the constraint that shapes the layout
of every package below `pw`.

**An untagged file is compiled into every build.** It may therefore name only
packages every build links. `pw` is not one of them: a file with no tag that
imports `pw` puts the whole net/http runtime into the fasthttp binary. Reach for
`pwruntime`, `pwconfig`, `pwsession`, `pwdatabase`, `pwobservability`,
`pwextension`, `pwratelimit` or `pwbrowser` instead — each publishes what `pw`
re-exports.

## Popcorn Wave

| Tag | What it selects | Who passes it |
|---|---|---|
| `fasthttp` | The second build. `pwfast` replaces `pw`, and the binary links no `pw` at all. Requires `project.fasthttp = true` in `popcornwave.toml`. | `pw build --backend fasthttp` |
| `pwdev` | The development halves: dev console, storybook, dev data, `--pw-print-dsn`. | `pw dev`, `pw storybook` and `pw migrate` pass `-tags=pwdev` to `go run` |
| `force_tinygo_logic` | Compiles the TinyGo code path under host Go, so it can be tested without TinyGo. Defined by tinygodriver; Popcorn Wave follows the convention in its own compression and migration splits. | You, in tests |
| `tinybind_no_openapi` | Keeps the generated OpenAPI fragments out of the build. Defined by tinybind-go, and appears in files `pw generate` writes. | You |

`pw_nozstd` and `pw_nogzip` were removed in favour of `middleware.compression`.
They dropped a response encoder back when zstd meant linking a decoder ten times
its size; the encoder is its own package now and the pair costs 387 KB, which is
less than the question was worth. Passing either is not an error — it selects
nothing.

## tinygodriver

| Tag | What it selects |
|---|---|
| `force_tinygo_logic` | The native backend under host Go, so it is testable without TinyGo. Paired with `tinygo` throughout: `!tinygo && !force_tinygo_logic` selects the standard path, `(tinygo \|\| force_tinygo_logic)` the native one. |
| `fasthttp_nozstd` | Drops `klauspost/compress/zstd` from the fasthttp fork. A TinyGo build **needs** this today; see below. |
| `darwinstarttlswith13` | Replaces both macOS TLS backends with vendored mbedTLS, buying TLS 1.3 and client certificates at the cost of the OS trust policy. |
| `nosqlite` | Keeps the SQLite amalgamation from linking. Without it, importing the package compiles SQLite in. |
| `jwt_no_rsa` | Drops RSA from the JWT package. |
| `nopgxregisterdefaulttypes` | Skips pgx's default type registration. An upstream pgx tag, carried through the vendored copy. |

## tinybind-go

| Tag | What it selects |
|---|---|
| `tinybind_no_openapi` | Excludes generated OpenAPI fragments. This is where the tag is defined; Popcorn Wave's generated files carry it. |
| `goexperiment.jsonv2` | A benchmark fixture only. Not something an application sets. |

## Tags the toolchain sets

These appear in constraints you will read, and you never pass them.

| Tag | Set by |
|---|---|
| `tinygo` | TinyGo |
| `gc` | The gc compiler — **and TinyGo** |
| `scheduler.threads` | TinyGo, derived from `-scheduler=threads` |
| `wasip2`, `illumos` | `GOOS` |
| `appengine` | Nothing current. Still honoured by `klauspost/compress` and the websocket fork as a pure-Go switch |

`gc` is worth knowing about because it is the one that surprises. TinyGo sets it,
so a dependency whose pure-Go fallback is guarded on `!gc` — a constraint written
for compilers that do not claim to be gc — selects its assembly under TinyGo and
fails to link.

## `-scheduler=threads` under TinyGo

This one is not a `-tags` value. TinyGo derives the `scheduler.threads` build tag
from its own `-scheduler` flag, which is what lets the framework make its absence
a compile error:

```bash
tinygo build -scheduler=threads -o app ./cmd/app
```

**Every database engine that speaks a network protocol requires it.** Under the
cooperative scheduler a blocking socket call holds the whole runtime, so the
driver's cancellation watcher never runs — a 5s server-side sleep under a 500ms
deadline returned after the full 5s, with a nil error and nothing logged.

`database/postgres` and `database/mysql` therefore refuse to compile without it.
The guard is keyed on the import graph, so it fires for exactly the programs that
link the engine, however the build was invoked, and the diagnostic is the name of
an identifier that does not exist:

```
undefined: build_this_program_with_tinygo_scheduler_threads
```

`pw build` does not drive TinyGo, so nothing passes the flag on your behalf at the
command line. The `Dockerfile.tinygo` that `pw init` writes already carries it.

## TinyGo and fasthttp together

Nothing beyond the target tag, as of tinygodriver v1.2.4:

```bash
tinygo build -tags fasthttp -scheduler=threads -o app ./cmd/app
```

Before v1.2.4 this needed `fasthttp_nozstd` as well, and without it the linker
could not resolve `klauspost/compress/zstd`'s arm64 assembly by the `gc`
mechanism above. Both unresolved symbols decoded — the net/http build always
linked, because it only encodes and dead-code elimination dropped the rest. The
fasthttp fork now encodes through tinygodriver's own zstd under TinyGo, so there
is no klauspost assembly left to reach.

`fasthttp_nozstd` still works and now saves about 40 KB, so there is little
reason to pass it. Drop `-scheduler=threads` only if the binary links no
network database driver; see the section above.

For what each build target costs at run time and on disk, see
[Build targets](/guides/architecture/performance/).
