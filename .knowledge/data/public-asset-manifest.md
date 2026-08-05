---
id: data:public-asset-manifest
type: data
title: Public Asset Manifest
---
One build-produced record per served URL, listing every representation behind it with the bytes, digest, and cache metadata api:public-asset-middleware answers with.

```yaml
scope: concept:derived-public-tree, every file under dist/public
entry:
  url: mount-relative slash path, the value a rewritten reference names
  source: authored path it came from, or absent for a generated artifact
  cache_control: the header to send, per requirement:derived-asset-pipeline caching
  representations:
    - path: file under dist/public
      media_type: Content-Type to send
      content_encoding: empty or zstd
      bytes: encoded length
      sha256: digest of the emitted bytes
      etag: quoted strong tag derived from that digest
      preference: build-declared order among media types of one URL
form:
  emitted: generated Go table beside the embed, so nothing parses a manifest at startup and TinyGo pays no decoder
  also_written: a JSON copy for api:cli-doctor and for a caller inspecting a build, never read at runtime
  determinism: identical inputs produce identical entries in a stable order, so policy:generated-artifacts --check compares it
rules:
  - a URL with no entry is 404 even when a file exists under dist/public, so serving is manifest-driven rather than filesystem-driven
  - an entry with no representation is a build error
  - each representation carries its own strong ETag, per policy:public-asset-negotiation
  - the digest is over emitted bytes, so an identity and a zstd representation never share a tag
  - the middleware computes no digest and reads no file metadata per request, which removes the current per-request sha256 and read
  - the manifest is embedded with the tree it describes and is never fetched or reloaded
  - a media-type set larger than one enables policy:public-asset-media-negotiation for that URL only
open_questions:
  - whether the local-override path of policy:public-asset-resolution keeps a manifest at all, or drops to today's per-request digest
  - whether an entry records its transform and settings, which would make a stale-output diagnostic possible
```
