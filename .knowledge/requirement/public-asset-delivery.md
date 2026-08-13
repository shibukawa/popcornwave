---
id: requirement:public-asset-delivery
type: requirement
title: Public Asset Delivery
---
The conventional project-root public directory becomes an embedded, optionally overlaid, externally reachable static asset tree.

```yaml
source: project-root public/
scaffolded_embed: project-root public.go
build: flow:public-asset-build
runtime: api:public-asset-middleware
configuration: data:server-runtime-config
development: decision:development-public-assets
rules:
  - public is the only framework-owned static directory convention
  - serving precompressed sidecars performs no runtime compression or decompression
  - ordinary application routes remain responsible for non-public content
  - application startup rejects mount collisions
  - disabling the endpoint registers no public route but does not change the built binary
open_questions:
  large_media_in_an_embedded_tree:
    raised: 2026-08-11, from the video case of policy:asset-content-signature
    fact: the api:cli-init scaffold is //go:embed all:dist/public, so an mp4 under public/ is compiled into the binary and is resident in its image
    why_it_is_not_only_a_size_note:
      - policy:public-asset-precompression already excludes audio and video, so the tree carries them once rather than twice, which is what makes the raw size the whole cost
      - requirement:cloudflare-workers-hosting checks the emitted upload against the active plan, so a video is a deploy failure there rather than a slow build; that check is per target and nothing warns the other ones
      - a TinyGo target under rule:tinygo-runtime-compatibility pays the same bytes with less room
    what_is_missing: no check anywhere reports an authored file whose size makes the embed the wrong mechanism for it
    candidate: a size advisory over the authored tree, since the walk of requirement:asset-content-verification already visits every file and knows its length
    open_part: what the threshold is, and whether the advice is object storage, a CDN, or only that the author should know; the framework has no media hosting story to point at yet
    not_urgent_because: the failure is loud where it is bounded, and elsewhere it is a large binary rather than a wrong one
    candidate_answer: requirement:external-public-assets, which keeps the URL and takes the bytes out of the binary, and which surfaced that the missing piece is range requests rather than size
    what_that_makes_this_check: with a second tree to name, the advisory acquires a remedy it did not have; it also becomes the whole of the size rule, since placement decides where bytes live and a threshold only decides whether to speak
references:
  - https://pkg.go.dev/embed
```
