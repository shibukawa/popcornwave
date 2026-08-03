---
title: Static Assets
description: The public directory — embedded into the binary, precompressed at build time, and served with ETags and content negotiation.
sidebar:
  order: 6
---

A project has a `public/` directory at its root. Whatever you put there is
served, and it travels inside the binary rather than beside it: one file to
deploy, and no chance of a stylesheet that belongs to yesterday's release.

```
public/
  favicon.ico
  generated/app.css
  images/logo.svg
```

```
GET /public/images/logo.svg
```

That is the whole setup. `pw init` writes the directory and the `public.go` that
embeds it, so a scaffolded project already serves from it.

## What embeds it

`public.go` is ordinary application code, not generated output, and it stays
readable:

```go
package publicassets

//go:embed all:public
var embeddedPublic embed.FS

func init() {
	middlewares.RegisterPublicFS(PublicFS())
}
```

The `init` registration is why nothing in your `main` mentions assets. The
framework picks the filesystem up during startup and mounts it. Serving with
`server.public.enabled` on and no registered filesystem is a startup error
rather than a directory that quietly 404s.

`public` is the only static directory convention the framework has. Anything
else you want to serve — a user upload, a generated report — is an ordinary
route, and [object storage](/guides/storage/object-storage/) is usually the
better home for it.

## Precompression at build time

[`pw build`](/pw/project/build/) writes a `.zstd` sibling next to every
compressible asset before it compiles. Serving then costs no CPU at all: the
encoded bytes already exist, and the request either gets them or gets the
original.

Eligibility is decided by media type, and covers what actually benefits:

| Compressed | Left alone |
| --- | --- |
| `.html`, `.css`, `.js`, `.mjs`, `.json`, `.map`, `.txt`, `.xml`, `.svg`, `.webmanifest`, and any other `text/*` | images other than SVG, audio, video, fonts, archives, WebAssembly — anything already compressed |

Sidecars are build artifacts. The scaffolded `.gitignore` excludes
`public/**/*.zstd`, and a build removes a sidecar whose source is gone, so a
deleted file cannot leave a stale representation behind.

A `.zstd` path is never a URL. It is an internal representation of the asset
beside it, and a request for one is a 404.

## What a request gets

| Behaviour | Detail |
| --- | --- |
| Methods | `GET` and `HEAD`; anything else is `405` with `Allow` |
| Mount without the trailing slash | `308` redirect to the mount, query string preserved |
| Encoding | `zstd` when `Accept-Encoding` explicitly allows it with a non-zero q-value *and* a sidecar exists; the original otherwise |
| `Vary` | `Accept-Encoding`, on every response that negotiated one |
| `ETag` | strong, derived from the bytes actually sent |
| `If-None-Match` | `304` on a match |
| Nothing acceptable | `406` |

The ETag is computed per representation, so the compressed and uncompressed
forms of one file never share a validator — a cache that stored one cannot serve
it to a client that asked for the other.

A directory resolves to its `index.html` when there is one, and to a `404`
otherwise. There are no directory listings.

## Path handling

The rules here are all refusals, and they are worth knowing because they are
absolute:

- dot-prefixed segments are rejected, so `.env` and `.git/config` under `public/`
  are not reachable
- traversal, backslashes, and NUL are rejected after one round of percent-decoding
- symbolic links are refused — the local root, anything below it, and any
  non-regular file. `pw build` refuses to walk one too, rather than embedding
  whatever it points at
- a `.zstd` suffix in the request path is rejected outright

## During development

[`pw dev`](/pw/project/dev/) builds in a reserved mode that reads `public/`
straight from disk on every request. Edit a stylesheet and reload — no
precompression, no rebuild, no restart. In that mode nothing negotiates
encoding, no sidecar is read or written, and a file you deleted returns `404`
even when an older copy is still embedded in the binary.

Outside that mode, `server.public.read_local = true` gives you the same
disk-first behaviour from an ordinary build, for the deployment that overlays a
directory onto the embedded tree.

## Configuration

| Key | Default | Meaning |
| --- | --- | --- |
| `server.public.enabled` | `true` | serve the assets at all |
| `server.public.mount` | `"/public"` | where they are mounted |
| `server.public.read_local` | `false` | prefer `./public` on disk over the embedded tree |

The mount must be an absolute, canonical, non-root path with no wildcards, and
an application route that collides with it fails startup rather than shadowing
it. Turning `enabled` off registers no route but changes nothing about the
binary — the assets are still embedded, just not reachable.

The framework's own browser scripts do not live here. They are served from a
fixed `/_pw/` prefix ahead of application routing, which is what keeps them
available no matter how this endpoint is configured. See
[Progressive rendering](/guides/cross-layer/async-rendering/).

## Not the same as response compression

The sidecars above are static files, compressed once at build time. Compressing
an HTML response the application just rendered is a separate switch with
separate trade-offs — see
[Response Compression](/guides/frontend/compression/). That middleware never
recompresses what this handler served.
