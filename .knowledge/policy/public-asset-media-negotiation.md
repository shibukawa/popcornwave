---
id: policy:public-asset-media-negotiation
type: policy
title: Public Asset Media Negotiation
---
A URL holding more than one media representation selects one from the request Accept header, independently of the Accept-Encoding selection policy:public-asset-negotiation already performs.

```yaml
scope:
  applies: an entry of data:public-asset-manifest whose representations differ in media_type, which today means only the avif case of policy:asset-transform-matrix
  never: an entry with one media type, which answers exactly as it does now
selection:
  - parse Accept media ranges and q-values, treating an absent header as accepting the build-declared default
  - consider only representations the manifest declares for that URL
  - among acceptable ones choose the highest build-declared preference, not the highest client q-value, because the ordering is a build judgment about bytes and the client only states capability
  - fall back to the default representation when nothing else is acceptable
  - 406 only when the default is explicitly unacceptable
response:
  Content-Type: the media type of the selected representation, which may disagree with the URL extension
  Vary: Accept beside Accept-Encoding
  ETag: the selected representation's tag, per data:public-asset-manifest
cache_key_bounds:
  problem: Accept is high-entropy, so a shared cache keyed on it stores one variant per client string
  rule: normalize the header to a bounded class before it influences selection, where a class is the set of manifest media types the request accepts
  effect: the variant space is the manifest's, not the header's
  operator_note: a CDN unable to normalize should be pointed at the single-representation shape instead, which is the default
composition:
  order: media type first, then content encoding, because a zstd sidecar exists per representation and not per URL
  independence: this policy and policy:public-asset-negotiation both run and neither reads the other's header
rules:
  - no conversion, transcoding, or image decoding happens at request time
  - GET and HEAD only, unchanged from api:public-asset-middleware
  - the development path serves the default representation and negotiates nothing, per decision:derived-tree-development
  - an entry declaring two representations with one media type is a build error, since the choice would be undefined
why_the_response_and_not_the_markup:
  alternative: distinct URLs for webp and avif behind an author-written picture element, which is honest about extensions and needs no Vary
  refused_as_a_build_output: policy:asset-transform-matrix never generates a picture, because changing the element tree can break css and structural javascript the build cannot see
  author_written: a picture the author wrote keeps working; its source URLs are left alone and only its img src is converted
  consequence: with markup fixed, a second media type has nowhere to be selected but here
references:
  - https://www.rfc-editor.org/rfc/rfc9110.html#name-accept
```
