---
title: Build tags
description: Every build tag defined across Popcorn Wave, tinygodriver and tinybind-go — what each one selects, who passes it, and which ones the toolchain sets for you.
sidebar:
  order: 9
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
| `fasthttp` | The second build. `pwfast` replaces `pw`, and the binary links no `pw` at all. Requires `project.fasthttp = true` in `popcornwave.toml`. | `pw build --target fasthttp` |
| `pwdev` | The development halves: dev console, storybook, dev data, `--pw-print-dsn`. | `pw dev`, `pw storybook` and `pw migrate` pass `-tags=pwdev` to `go run` |
| `pw_nozstd` | Removes the zstd response encoder, worth about 247 KB. | You |
| `pw_nogzip` | Removes the gzip response encoder. | You |
| `force_tinygo_logic` | Compiles the TinyGo code path under host Go, so it can be tested without TinyGo. Defined by tinygodriver; Popcorn Wave follows the convention in its own compression and migration splits. | You, in tests |
| `tinybind_no_openapi` | Keeps the generated OpenAPI fragments out of the build. Defined by tinybind-go, and appears in files `pw generate` writes. | You |

Both codec tags exist for the same deployment: something in front of the
application already compresses, so the encoder is code that is linked and never
runs. Turning one off does not misreport the result — the build stops offering
that coding rather than advertising one it cannot produce.

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
| `wasip2`, `illumos` | `GOOS` |
| `appengine` | Nothing current. Still honoured by `klauspost/compress` and the websocket fork as a pure-Go switch |

`gc` is worth knowing about because it is the one that surprises. TinyGo sets it,
so a dependency whose pure-Go fallback is guarded on `!gc` — a constraint written
for compilers that do not claim to be gc — selects its assembly under TinyGo and
fails to link.

## TinyGo and fasthttp together

A TinyGo build of the fasthttp target needs `fasthttp_nozstd`, and nothing passes
it for you yet:

```bash
tinygo build -tags "fasthttp fasthttp_nozstd" -o app ./cmd/app
```

Without it the linker cannot resolve `klauspost/compress/zstd`'s arm64 assembly,
by the `gc` mechanism above. Both missing symbols decode; the net/http build
links because it only encodes and TinyGo's dead-code elimination drops the rest.

`-tags noasm` also links, by swapping klauspost's assembly for its pure-Go
fallback, but it keeps zstd and costs about 2.5 MiB more. Prefer
`fasthttp_nozstd`.

One thing to know before you drop it: `middleware.compression_codings` defaults
to `zstd,gzip`. A build without zstd serves those clients identity instead, which
is correct but is not what the setting says — narrow the default if it matters to
you.

For what each build target costs at run time and on disk, see
[Build targets](/guides/architecture/performance/).
