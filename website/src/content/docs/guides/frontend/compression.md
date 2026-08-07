---
title: Response Compression
description: One switch that zstd-encodes rendered HTML, what it covers, and what it costs on a streamed page.
sidebar:
  order: 7
---

```toml
[middleware]
compression = true
```

That is the whole interface. With it on, HTML the application renders is
zstd-encoded for clients that accept it.

It is off by default because something in front of the application often
compresses already, and encoding a body twice is work nobody benefits from.
Turn it on when nothing else is doing it.

## What it covers

The switch applies to rendered HTML, and only to that:

| Compressed | Not compressed |
| --- | --- |
| `WriteHTML`, `WriteHTMLPage`, `WriteHTMLChain` | `WriteAPI` |
| `WriteHTMLFragment` | `WriteProblem` |
| the streamed branch of a page with await boundaries | [static assets](/guides/frontend/static-assets/), which carry their own precompressed sidecars |

A response that already has a `Content-Encoding` is left as it is. Nothing
encodes twice.

`zstd` is the only encoding offered. There is no gzip fallback, so a client that
does not advertise `zstd` gets the original bytes — which is the correct
outcome, not a degraded one.

## What the response looks like

`Vary: Accept-Encoding` is set whether or not the body ends up encoded. This
matters more than it looks: a cache that stored one representation must not hand
it to a client that asked for the other, and the header is what tells it so.

When the client does accept `zstd` with a non-zero q-value:

- `Content-Encoding: zstd`
- no `Content-Length` — the encoder knows the length only after it closes
- no `ETag`. The hash is readable only after `Close`, by which time the headers
  are long gone. Static assets get validators; dynamically rendered HTML does
  not

## Streaming costs something

A page with [await boundaries](/guides/cross-layer/async-rendering/) commits its shell
first and sends each region as it settles. Compression follows it there, and the
encoder is flushed once per settled boundary so a completion reaches the browser
instead of waiting in a buffer.

Flushing ends a block, and a block ended early compresses worse. **A streamed
page compresses worse than the same page buffered.** That is the trade, and it
is usually worth it — the first paint arrives sooner either way.

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

There is one boolean. No minimum size, no content-type list, no encoder level.
A short response is encoded like a long one, and the encoder runs at its default
setting.

If you need those knobs, the honest answer is that a reverse proxy or CDN in
front of the application already has them, and it is the better place for this
job. This switch exists for the deployment that has no such layer.

## Removing the encoder from the binary

`compression = false` stops the encoder from running. It does not stop it from
being compiled in: the zstd encoder is linked into every build whether the
switch is on or off, and it costs roughly 400 KB of binary. A runtime value
cannot unlink code, which is why the switch has a build-time counterpart:

```bash
go build -tags pw_nozstd ./cmd/yourapp
```

With the tag, the encoder is never compiled and every response leaves
unencoded, exactly as with `compression = false`. Setting `compression = true`
in configuration then does nothing — the build has no encoder to turn on, and
that is a silent no-op rather than a startup error, so decide at build time and
keep the configuration consistent with it.

Use the tag when both halves hold: binary size matters for your target — a
container image pulled on every deploy, an edge runtime with a size budget —
and compression is terminated at a CDN or reverse proxy anyway, so the encoder
was dead weight. If the application serves compressed HTML to browsers itself,
keep the default build. 400 KB is rarely worth losing the capability.
