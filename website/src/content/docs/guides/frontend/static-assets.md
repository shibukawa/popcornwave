---
title: Static Assets
description: The public directory is what you write; a build turns it into the tree that ships, with a manifest that decides every cache header before a request arrives.
sidebar:
  order: 7
---

A project serves static files from the `public/` directory at its root. The
files are embedded in the binary, so the executable and its assets are deployed
together.

```
public/                     what you write
  favicon.ico
  generated/app.css
  images/logo.png
```

```
GET /public/images/logo.png
```

`pw init` creates both the directory and the `public.go` file that embeds it, so
a scaffolded project can serve assets immediately.

The binary actually embeds `dist/public`, not the source directory.
[`pw build`](/pw/project/build/) copies or transforms files from `public/` into
that build tree. With no transformations the copy is byte-for-byte identical;
when a transformation renames a file, the build tree holds the result and the
reference rewrite follows it. Serve assets elsewhere when a CDN or object store
needs to manage them independently from application releases.

## What ships

```go
package publicassets

//go:embed all:dist/public
var embeddedPublic embed.FS

func init() {
	middlewares.RegisterPublicFS(PublicFS())
}
```

`public.go` is ordinary application code, not generated output. The `init`
registration is why nothing in your `main` mentions assets: the framework picks
the filesystem up during startup and mounts it. Serving with
`server.public.enabled` on and no registered filesystem is a startup error
rather than a directory that quietly 404s.

Beside it the build writes `public_manifest_pw_gen.go`, which is generated: one
entry per URL, naming every representation, its length, its validator, and its
cache policy. A response reads bytes and computes nothing. A URL the build did
not declare is a `404` whatever the tree happens to hold, which is what stops a
file that slipped in from being reachable without a build having decided it
should be.

`public` is the only static directory convention the framework has. Anything
else you want to serve — a user upload, a generated report — is an ordinary
route, and [object storage](/guides/storage/object-storage/) is usually the
better home for it.

## Two cache policies, decided by the name

A file that keeps the name you wrote gets `public, no-cache` and a strong
`ETag`. The browser revalidates and an unchanged asset costs a `304` with no
body. That is as much as a stable name can honestly promise, because the next
build may put different bytes behind it.

A file the build *produced* is named after the digest of its own bytes —
`logo.4f2a91c07b3e.webp` — and gets `public, max-age=31536000, immutable`.
Different bytes are a different URL, so the promise is true rather than hopeful.

Only produced files are named that way, and the reason is worth stating: hashing
a name works only where every reference to it is rewritten. The build rewrites
the `src` of an `img` and of a built `script`, and the `url()` of a stylesheet.
It does not rewrite a `link href`, so a stylesheet keeps its name and
revalidates.

## Conversion

Nothing converts until you ask. [`pw add images`](/pw/project/add/) installs the
encoders and the switch together, which is deliberate: a switch without the
tools converts nothing and says so, and neither one alone is useful.

```toml
[assets.images]
enabled = true
quality = 75
avif = false

[assets.css]
minify = true

[assets.scripts]
enabled = true
```

What each one does, and what follows the file:

| Source | Becomes | The reference |
| --- | --- | --- |
| `img src` naming a `.png` or `.jpg` | WebP, lossless from a PNG and lossy from a JPEG | rewritten to the hashed name |
| `script src` naming a `.ts` or `.tsx` | a bundled ES module, with a source map | rewritten to the hashed name |
| a `.css` file | minified, with its `url()` references pointed at whatever they became | unchanged — the stylesheet keeps its own URL |
| a `.js` file | minified, not bundled, so a module stays a module | unchanged |
| anything else | copied | unchanged |

A conversion that loses is declined. An encode that came out larger than its
source leaves the reference on the original and says why in the build output,
because a derived file nobody benefits from is weight with extra steps.

The authored source is dropped from the shipped tree once every reference the
build can see has been rewritten. When one it cannot rewrite remains — a path in
a `meta` tag, a URL a script builds — the source is kept and the build says so.
That way a page never loses an image to a conversion it did not know about.

With the script build on, TypeScript is an input rather than a file the tree
owes anyone, so a module an entry imported is not served either. No browser runs
it, and the source map already carries its text, so a stack trace still names the
authored line. The same retention rule applies: a `.ts` some Go code still names
stays, and the build says why.

### A built script needs `type="module"`

The build emits a module, and a module under a classic `<script src>` is a
syntax error at load time: the page renders and silently loses its script. The
build refuses that rather than shipping it, and names the template and line.

```html
<script type="module" src="/public/js/app.ts"></script>
```

### AVIF is chosen per request

With `avif = true` an image carries a second representation behind the same URL,
and `Accept` decides which one a request gets. `Vary` names `Accept` only for
the URLs that actually have two, so a cache stores one variant for everything
else.

The build never writes a `<picture>` element for you. Wrapping an `img` changes
the element tree, which can break a CSS combinator or a script that walks the
DOM, and neither of those is visible from here. Writing one yourself works fine
— its `img src` converts like any other, and its `source` elements are left
alone.

## Precompression

The build writes a `.br`, a `.zstd` and a `.gz` sibling next to every
compressible file after the conversions, so what is compressed is what actually
ships. Serving then costs no CPU at all: the encoded bytes already exist.

| Compressed | Left alone |
| --- | --- |
| `.html`, `.css`, `.js`, `.mjs`, `.json`, `.map`, `.txt`, `.xml`, `.svg`, `.webmanifest`, and any other `text/*` | images other than SVG, audio, video, fonts, archives, WebAssembly — anything already compressed |

All three run at their maximum level, which is affordable here for the reason it
is not on [a rendered response](/guides/frontend/compression/): the cost lands on
the build rather than on a request. Brotli exists only here, and only because of
that — at maximum it comes out roughly fifteen percent smaller than zstd and
seventeen percent smaller than gzip, a margin that appears at levels far too slow
to encode while a client waits.

A coding whose output is not smaller than its source is skipped rather than
written, so a short file may have fewer than three siblings, or none. That is
ordinary: negotiation falls through to the next coding, and the identity bytes
always answer.

A sidecar path is never a URL. It is a representation of the asset beside it, and
a request for one is a `404`. Each form carries its own validator, so a cache that
stored one cannot serve it to a client that asked for another.

## What a request gets

| Behaviour | Detail |
| --- | --- |
| Methods | `GET` and `HEAD`; anything else is `405` with `Allow` |
| Mount without the trailing slash | `308` redirect to the mount, query string preserved |
| Encoding | the first of `br`, `zstd`, `gzip` that `Accept-Encoding` allows with a non-zero q-value *and* has a sidecar; identity otherwise. The order is the build's, smallest first, not the client's q-values |
| Media type | the preferred representation the request accepts; the fallback otherwise |
| `Vary` | `Accept-Encoding`, plus `Accept` where a URL has more than one media type |
| `ETag` | strong, from the build, per representation |
| `If-None-Match` | `304` on a match |
| Everything refused | `406` |

A directory resolves to its `index.html` when there is one, and to a `404`
otherwise. There are no directory listings.

## Path handling

The rules here are all refusals, and they are worth knowing because they are
absolute:

- dot-prefixed segments are rejected, so `.env` and `.git/config` under `public/`
  are not reachable
- traversal, backslashes, and NUL are rejected after one round of percent-decoding
- symbolic links are refused — the local root, anything below it, and any
  non-regular file. The build refuses to walk one too, rather than embedding
  whatever it points at
- a `.br`, `.zstd` or `.gz` suffix in the request path is rejected outright

## During development

[`pw dev`](/pw/project/dev/) runs the same conversions and serves `dist/public`
from disk. Edit an asset and reload: the tree is rebuilt, and no Go rebuild
happens for a file the binary does not compile.

It has to run the same conversions rather than skip them, because a rewritten
reference is compiled into generated code — a development build that skipped
them would serve pages naming files it never produced. What keeps that
affordable is the conversion cache under `dist/`: an unchanged asset costs a
digest instead of an encode. The first build after a clone is the one that pays.

Outside that loop, `server.public.read_local = true` gives the same disk-first
behaviour from an ordinary build, for the deployment that overlays a directory
onto the embedded tree.

## Configuration

| Key | Default | Meaning |
| --- | --- | --- |
| `server.public.enabled` | `true` | serve the assets at all |
| `server.public.mount` | `"/public"` | where they are mounted |
| `server.public.read_local` | `false` | prefer the built tree on disk over the embedded one |

The mount must be an absolute, canonical, non-root path with no wildcards, and
an application route that collides with it fails startup rather than shadowing
it. Turning `enabled` off registers no route but changes nothing about the
binary — the assets are still embedded, just not reachable.

The build-time keys are in
[Build Tool Configuration](/reference/build-configuration/), and `dist/` is
build output: the scaffolded `.gitignore` excludes everything under it except
the sentinel that keeps the embed compiling before the first build.

The framework's own browser scripts do not live here. They are served from a
fixed `/_pw/` prefix ahead of application routing, which is what keeps them
available no matter how this endpoint is configured. See
[Progressive rendering](/guides/cross-layer/async-rendering/).

## Not the same as response compression

The sidecars above are static files, compressed once at build time. Compressing
a response the application just rendered is a separate switch with separate
trade-offs — see [Response Compression](/guides/frontend/compression/). That
middleware never recompresses what this handler served.

The two also offer different codings, and for the same reason the levels differ:
a rendered response is encoded while a client waits, so it offers only `zstd` and
`gzip` and runs them shallow. Brotli and the maximum levels stay here, where
nobody is waiting on them.
