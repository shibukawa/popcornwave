---
title: Response Compression
description: One switch that encodes rendered HTML and JSON, which codings it offers, and what it costs on a streamed page.
sidebar:
  order: 7
---

```toml
[middleware]
compression = true
```

That is the whole interface for most deployments. With it on, the HTML and JSON
the application renders are encoded for clients that accept it.

It is off by default because something in front of the application often
compresses already, and encoding a body twice is work nobody benefits from.
Turn it on when nothing else is doing it.

## What it covers

The switch applies to what the application renders while a request waits:

| Compressed | Not compressed |
| --- | --- |
| `WriteHTML`, `WriteHTMLPage`, `WriteHTMLChain` | `WriteProblem` |
| `WriteHTMLFragment` | [static assets](/guides/frontend/static-assets/), which carry their own precompressed sidecars |
| `WriteAPI` | |
| the streamed branch of a page with await boundaries | |

A response that already has a `Content-Encoding` is left as it is. Nothing
encodes twice.

`WriteProblem` is the deliberate exception. A problem document is a few hundred
bytes built by hand on a path that must not fail, and every coding makes a body
that small *larger* — the frame header alone outweighs anything there is to
save. An encoder there would add a way to fail in exchange for nothing.

## Which codings

Two are offered, and `zstd` is tried first:

```toml
[middleware]
compression = true
compression_codings = ["zstd", "gzip"]
```

That is the default, so you only write the second line to change it. The order
is the deployment's, not the client's: a `q`-value in `Accept-Encoding` says
what the client *can* read, and which of two readable codings is worth spending
CPU on is not the client's judgement to make. A `q=0` still excludes, because
that half of the header is a statement about capability.

A coding left out of the list is not offered at all, even to a client asking for
it, so the one key expresses removal as well as order. To turn compression off
entirely, use `compression = false` rather than an empty list.

**Leave the order alone unless you have measured your own traffic.** zstd leads
because it stays slightly ahead of gzip on ratio at the levels both run, and
because a client already receiving zstd keeps receiving it. Reverse it only if
your clients are overwhelmingly ones that would take gzip anyway and you would
rather not link the zstd encoder at all — in which case the build tag below is
the more direct answer.

gzip is what makes this list worth having. It is the one coding no browser,
proxy, crawler, or command-line client lacks. Safari in particular advertises
`zstd` only from Safari 26 on macOS Tahoe and Safari 26.3 on iOS, and that
support comes from the operating system's network stack rather than the browser,
so it arrives at the speed of OS upgrades. Before gzip was offered here, those
clients received the original bytes.

Brotli is not on the list. It belongs to the [static asset
build](/guides/frontend/static-assets/), where the encoder runs once at build
time. Per request it is the wrong trade: at a level fast enough to serve, it
costs roughly four times the CPU of zstd to remove another eight percent of the
body, and its pure-Go encoder is worth about 795 KB of binary — three times the
zstd one, in a framework that ships a build tag to remove that.

## What the response looks like

`Vary: Accept-Encoding` is set whether or not the body ends up encoded. This
matters more than it looks: a cache that stored one representation must not hand
it to a client that asked for another, and the header is what tells it so.

When a coding is selected:

- `Content-Encoding: zstd` or `gzip`
- no `Content-Length` — the encoder knows the length only after it closes
- no `ETag`. The hash is readable only after `Close`, by which time the headers
  are long gone. Static assets get validators; dynamically rendered responses do
  not

## Streaming costs something

A page with [await boundaries](/guides/cross-layer/async-rendering/) commits its shell
first and sends each region as it settles. Compression follows it there, and the
encoder is flushed once per settled boundary so a completion reaches the browser
instead of waiting in a buffer.

Flushing ends a block, and a block ended early compresses worse. **A streamed
page compresses worse than the same page buffered.** That is the trade, and it
is usually worth it — the first paint arrives sooner either way. It holds for
both codings; nothing about it is specific to which one was negotiated.

There is a subtler cost. Per-boundary flushing makes the compressed length of
each region observable on its own, which is a finer-grained oracle than one
length for the whole page. If a boundary renders a secret next to input an
attacker controls, do not stream it. Render that page buffered:

```toml
[html]
streaming = false
```

The same key is the workaround for a proxy that buffers encoded responses and
defeats progressive delivery anyway.

## What is not configurable

Whether to encode, and in what order. Nothing else: no minimum size, no
content-type list, and in particular no encoder level. A short response is
encoded like a long one.

The level is fixed rather than exposed because it answers to a measured cliff
that does not move between deployments. A body here is encoded while a request
waits, so throughput is the scarce resource and ratio is the one that gives:
gzip runs at level 1 and zstd at its fastest setting, which put both near thirty
percent of the source at roughly 450 MB/s. Level 6 gzip would buy three more
points of ratio for a third of the throughput, and level 9 for a *ninth* of it.
The deep levels are not lost — the asset build spends them, where the cost lands
on a machine that is not answering a request.

If you need the other knobs, the honest answer is that a reverse proxy or CDN in
front of the application already has them, and it is the better place for this
job. This switch exists for the deployment that has no such layer.

## Removing an encoder from the binary

`compression = false` stops the encoders from running. It does not stop them
from being compiled in: a runtime value cannot unlink code, which is why the
switch has build-time counterparts. There is one tag per coding.

```bash
go build -tags pw_nozstd ./cmd/yourapp
```

zstd is the one worth removing first, at roughly 247 KB against gzip's 148 KB
beside it. With the tag, that encoder is never compiled and the negotiation
stops offering the coding — a configuration file still naming `zstd` keeps
starting, because the build's decision has to win over the file rather than turn
a working configuration into a startup failure on a smaller target. The startup
log says which codings the binary could not produce, so this is visible rather
than silent.

Passing both tags leaves nothing to negotiate:

```bash
go build -tags pw_nozstd,pw_nogzip ./cmd/yourapp
```

Every response then leaves unencoded, exactly as with `compression = false`, and
setting `compression = true` does nothing. Decide at build time and keep the
configuration consistent with it.

Use the tags when both halves hold: binary size matters for your target — a
container image pulled on every deploy, an edge runtime with a size budget — and
compression is terminated at a CDN or reverse proxy anyway, so the encoders were
dead weight. If the application serves compressed responses to browsers itself,
keep the default build. A few hundred kilobytes is rarely worth losing the
capability.
