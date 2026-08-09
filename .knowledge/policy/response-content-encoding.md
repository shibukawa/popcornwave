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
  never:
    - api:problem-response, whose document is a few hundred hand-built bytes on a path that must not fail; every coding makes a body that size larger, so an encoder there buys a failure mode and no bytes. An HTML error page still encodes, because it goes out through api:html-response
    - api:public-asset-middleware, which answers from build-time representations per policy:public-asset-negotiation and performs no runtime encoding
    - requirement:package-asset-delivery, whose bytes are served as stored
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
