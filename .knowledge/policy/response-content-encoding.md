---
id: policy:response-content-encoding
type: policy
title: Response Content Encoding
---
How a dynamically rendered response picks a content coding, stated once for every writer that produces bytes at request time.

```yaml
scope:
  applies:
    - api:html-response, both the buffered and the streaming branch
    - api:html-fragment-response
    - api:api-response
    - requirement:navigation-delta-rendering, the streamed record body
    - api:live-delivery-protocol, from its first delivery rather than from its headers
    - requirement:reloadable-component-endpoint and requirement:action-response-update, whose bodies are assembled before they are written
    - the sequence tree of requirement:navigation-delta-rendering, which is the most compressible body on this wire and the one an edge holds longest
  never:
    - api:problem-response, whose document is a few hundred hand-built bytes on a path that must not fail; every coding makes a body that size larger, so an encoder there buys a failure mode and no bytes. An HTML error page still encodes, because it goes out through api:html-response
    - an update refusal, for the same reason and by the same rule: it says why one request failed and nothing else may be answered with it
    - api:public-asset-middleware, which answers from build-time representations per policy:public-asset-negotiation and performs no runtime encoding
    - requirement:package-asset-delivery, whose bytes are served as stored
  the_update_wire_was_the_gap:
    what: every update response wrote to the raw ResponseWriter while the document it replaces went through this negotiation
    cost: measured at 4156 bytes against an encoded 879 byte document on a page of 25 rows, so asking for a delta transferred nearly five times what reloading the page did
    why_it_hid: the comparison in the transfer measurement read unencoded bytes on both sides, where the delta wins by 2.4x; the asymmetry only exists once a coding is on, which is exactly the configuration a deployment serving traffic runs
    also_true_behind_a_proxy: a reverse proxy compressing text/html by default does not compress application/x-ndjson, so delegating the coding did not close it either
codings:
  offered: whatever data:compression-runtime-config lists, from the set zstd and gzip
  default_order: zstd, then gzip
  levels: shallow on both and not configurable, per requirement:response-gzip-encoder and requirement:contrib-zstd, because a dynamic body is encoded while a request waits
  why_zstd_leads_by_default:
    - zstd stays ahead of gzip on ratio at the levels both run, at 30.0 percent against 30.8, so leading with it costs the client nothing
    - a client receiving zstd today keeps receiving it, which makes gzip purely additive and the change unobservable to anyone it does not help
    - it is the newer format and the one worth being on by default, which is a direction and not a measurement
  never_offered:
    brotli: build-time only, per policy:public-asset-precompression; no served binary links an encoder
    deflate: refused, per decision:response-content-codings
selection:
  - parse Accept-Encoding tokens, wildcards, and q-values across every repetition of the header
  - a coding with q equal to zero is unacceptable, and so is one reached only through a wildcard whose q is zero
  - among acceptable codings choose by the configured order, not by client q-value, matching how policy:public-asset-media-negotiation treats preference
  - a coding absent from the configured list is not offered even when the client asks for it
  - absent, empty, or wholly unacceptable Accept-Encoding selects identity
  - identity is always available, so this path never answers 406
short_circuit:
  - a response already carrying Content-Encoding is left alone, so nothing is encoded twice
  - data:compression-runtime-config disabled selects identity without parsing the header
  - a coding whose encoder is absent from the build is not offered, per requirement:response-gzip-encoder and requirement:contrib-zstd
response:
  Content-Encoding: the selected coding, absent for identity
  Content-Length: removed, because a length is known only after the encoder closes
  ETag: not generated, since the digest is readable only after Close and the headers are long sent
  Vary: Accept-Encoding, added whether or not the body was encoded, so a cache holding one representation cannot answer a request that asked for another
streaming:
  - both codings flush per settled boundary and never per Write, per decision:streaming-response-compression
  - a flush ends a block, so a streamed page compresses worse than the same page buffered, for either coding
  - the per-boundary length oracle described by decision:streaming-response-compression is a property of chunked encoding and not of the coding, so it applies unchanged to gzip
minimum_size:
  rule: a body whose length is known before the frame opens is sent as it stands below 512 bytes
  why: every coding has a frame and a dictionary built from a few hundred bytes has nothing to say, so a small body comes out larger than it went in
  where_it_applies: the assembled update bodies only — a sequence tree, a redraw, an action response
  where_it_cannot: every streaming writer, whose length is unknown at the moment the frame has to be opened; a live stream that ends before its first delivery is covered instead by opening no frame until it commits
  not_configurable: it answers a property of the formats rather than of a deployment
configuration:
  surface: the two fields of data:compression-runtime-config, on and the coding order
  not_configurable: encoder level, minimum size, and content-type lists
  reason: a deployment wanting those knobs has a reverse proxy or CDN in front that already carries them, and this switch exists for deployments with no such layer
  why_the_order_is_the_exception: an operator can see their own client mix and CPU budget and the framework cannot, whereas a level is answered by a measured cliff that does not vary by deployment
rules:
  - one negotiation implementation serves every writer in scope, so an added coding reaches all of them at once
  - an aborted encoder discards its frame without committing bytes, preserving the api:problem-response fallback; this holds only because construction writes nothing, which is a requirement of every backend rather than a property they happen to share
  - a coding is offered only where the build can both encode and flush it
  - a list entry may hold several comma-separated names, so the same order set from one environment variable and from a TOML array reach negotiation identically
  - an encoder that fails to construct takes its Content-Encoding header back off, so the identity bytes that follow are not labelled with a coding nothing produced
references:
  - https://www.rfc-editor.org/rfc/rfc9110.html#name-accept-encoding
```
