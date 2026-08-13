---
id: requirement:asset-content-verification
type: requirement
title: Asset Content Verification
---

A static file is refused when its bytes contradict the type its extension declares, and an SVG that reaches a browser is neutralised whether or not the build recognised it, so nothing downstream of requirement:public-asset-delivery has to trust a name the content never earned.

```yaml
priority: should
status: implemented 2026-08-11
as_built:
  detection: internal/assetverify, shared because the build and the middleware both need it and sit on opposite sides of the module
  table: signature per format with per-offset parts, so RIFF and the ISO base media box are expressed rather than special-cased
  isobmff: the brand at offset 8 is deliberately not read, so every extension in the family accepts every brand; policing a growing registry would refuse a valid file the day a new brand ships
  build: the authored tree walk of api:cli-build, on bytes it already holds to digest them
  doctor: PW0130 and PW0131, reading only the window except for an svg
  request: the manifest-less path of api:public-asset-middleware, decided once beside the media type so a memoized asset costs no second look
  header: Content-Security-Policy sandbox added, never set, on an image/svg+xml response
  config: assets.verify enabled, svg_scan, and allow in data:project-config; server.public.svg_sandbox in data:server-runtime-config
  corrected_during_build: a signature-less extension over signature-less bytes first read as Confirmed, which would have claimed the table checked something it cannot; only a non-empty declared format can confirm
motivation:
  - static delivery decides everything from the extension: policy:asset-transform-matrix dispatches its transform on it, data:public-asset-manifest takes its media type from it through derivedMediaType, and api:public-asset-middleware sends that media type
  - so a file whose bytes disagree with its name is labelled by the name, and every response asserts a type the bytes never had
  - the motivating case is a .png holding script-bearing svg, which is served as image/png and becomes a different thing wherever the extension is trusted again
  - its sibling is the same file with an honest name, since a real .svg is served as image/svg+xml and does execute in the application origin
threat:
  what_nosniff_already_covers: policy:security-response-headers sends X-Content-Type-Options nosniff, so a browser fetching that .png through an img will not execute it
  what_it_does_not_cover: every consumer that re-derives the type from the name rather than the header, which is a cdn or object store re-serving the tree, a proxy, a person saving and reopening the file, and any tool reading the extension
  build_side: the transform dispatches on the extension too, so a mislabelled file either fails its encoder mid-build with an error about the wrong thing, or passes through unconverted and ships
  conclusion: the response header is already honest; the file is not, and the file is what gets embedded and shipped
conditions:
  mislabelled_file:
    asks: do the bytes match the name
    detection: policy:asset-content-signature
    diagnosis: PW0130
  active_svg:
    asks: does an honest file carry something that executes
    detection: policy:svg-active-content
    diagnosis: PW0131
  why_two: one switch over both would make a project disable the half it needed, and the two have different confidence; the first is decidable from bytes and the second is best effort behind a header that does not depend on it
subject:
  authored: the public tree the project wrote, which is the only tree whose names are a claim rather than a build output
  local_read: the same tree read per request when server.public.read_local is true, which no build validated
  excluded: concept:derived-public-tree output, per the exemption in policy:asset-content-signature
phases:
  build:
    where: the api:cli-build tree walk, which already reads every file to digest it
    severity: error, failing the build before anything is embedded
    reason: an embedded binary cannot be corrected at runtime, so the last moment to refuse is the one before the embed
  development:
    where: decision:derived-tree-development, which converts and serves from disk
    severity: error for that file only, answered 500 and reported through requirement:dev-error-overlay
    reason: dev and production agree that the file is not servable, without one bad asset ending the loop
  doctor:
    where: rule:project-integrity-checks
    severity: error
    reason: it reports without a build, and it is the only surface that sees a local_read tree
  request:
    mislabelled: the api:public-asset-middleware manifest-less path only, 500 for that file and logged once per path, since that is the one path serving bytes no build declared
    active_svg: the sandbox header of policy:svg-active-content, on every image/svg+xml response including manifest-driven ones
    unchanged: a manifest-driven response still verifies no content per request; the svg header is a constant chosen from the media type the manifest already carries
configuration:
  binding: data:project-config assets.verify
  keys:
    enabled: the signature check, default true
    allow: the shared path-glob exemption of policy:asset-content-signature
    svg_scan: the best-effort build scan, default true
    svg_sandbox: the response header, default true
  reason: the checks read bytes the walk already holds and the header is one field, so there is no cost to default them on; the switches exist for a project shipping a file the table judges wrongly, and for one deliberately serving interactive svg
scope_bound:
  no_text_parsing: neither condition parses a text format, per the rejected tier in policy:asset-content-signature and the not_detected list of policy:svg-active-content
  reason: a parser per format is a large surface for a narrow gain, and the svg half is already answered by a header that costs nothing and misses nothing
  what_that_gives_up: a malformed but honestly-named text file ships, and a script hidden behind encoding, smil, or a namespace prefix is not named at build time
  what_it_does_not: it is still refused at the browser, because the sandbox header does not depend on the build having understood the file
acceptance:
  - a public tree whose every file matches its extension builds byte-identically to today and adds no manifest entry or per-request work
  - a .png holding svg fails api:cli-build, naming the path, the declared type, and the detected one
  - the same file is a 500 rather than a 200 under pw dev and under server.public.read_local, and is reported once
  - a .css holding zip bytes fails, so a signature-less extension is not a hole
  - an .svg, a .css, and a .map with ordinary content pass silently, because a format with no signature produces no finding on its own
  - a malformed .svg that is honestly named and carries no matched literal ships, which is the deliberate limit rather than a miss
  - an avif variant, a .br sidecar, and a converted .webp never produce a finding, because build output is not the subject
  - a file matched by assets.verify.allow ships and is named in the build report
  - pw doctor reports the same two conditions the build fails on, with no build
  - an .svg containing a script element fails the build, naming the literal and its offset
  - every image/svg+xml response carries the sandbox policy beside the application one, and an svg used through an img renders exactly as before
  - an svg whose script the scan cannot see still runs in a unique origin, so the gap between the two halves is not an exposure
open_questions:
  isobmff_brand:
    what: every extension in the ISO base media family accepts every brand, so a .mp4 holding AVIF is confirmed
    why_it_was_taken: the brand registry keeps growing, and policing it refuses a valid file the day a new brand ships; the box already answers whether this is a media container at all
    reconsider_when: a brand set can be sourced rather than hand-copied, or a case appears where the family check is demonstrably not enough
    pinned_by: the test asserting a brand is not policed, so reversing this is a decision rather than an edit
  video_belongs_elsewhere:
    what: whether video should be in the embedded tree at all, which is a larger question than what its signature says
    where: the large_media_in_an_embedded_tree question of requirement:public-asset-delivery
    effect_here: if video moves out of public/, the ISOBMFF entry above stops mattering rather than needing a better answer
non_goals:
  - sanitising, rewriting, or stripping anything from a file
  - validating a file the build produced, which is checked by construction
  - parsing any text format, per scope_bound
  - inferring a media type from content, which stays the existing derivedMediaType fallback for an extension mime knows nothing about
  - serving svg from a separate origin, refused in policy:svg-active-content as a deployment the framework cannot provision
```
