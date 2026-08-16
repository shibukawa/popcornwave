---
id: requirement:localized-assets
type: requirement
title: Localized Assets
---
An asset that differs by language is a separate file at a separate URL, addressed by the locale tag rather than negotiated.

```yaml
status: proposed
problem: a title image, a banner, or a diagram carrying text has a per-locale version, so an asset is not always language-independent
layout:
  form: a locale-named directory under the asset root
  example:
    - public/assets/ja/hero.png
    - public/assets/en/hero.png
    - public/assets/logo.png
  reference: "<img src=\"/assets/{langtag}/hero.png\">, using the tag binding of decision:explicit-locale-in-links"
  why_the_tag_and_not_the_segment: the asset URL carries the language in every mode, including modes whose page URLs do not
existing_machinery_is_enough:
  manifest: data:public-asset-manifest gains ordinary entries
  signature: policy:asset-content-signature hashes per file, so variants differ naturally
  revision: policy:public-asset-revision is per file and needs no locale axis
  conclusion: no change to requirement:public-asset-delivery
negotiation_refused:
  shape: one URL answering with different bytes by Accept-Language
  why_not:
    - policy:public-asset-negotiation already negotiates media type, and a second axis multiplies the combinations
    - an asset carrying a Vary loses CDN efficiency, which is most of why assets are served the way they are
    - a localized asset has a stable address under the chosen layout, which nothing else needs
completeness_check:
  rule: a locale-named directory under an asset root is a localized set, and every declared locale supplies every member
  reported_by: rule:locale-prefix-checks
  reason: the image counterpart of the missing-translation check requirement:application-i18n exists for
  fallback: absent by default, so a gap is reported rather than silently answered by another locale's image
consequence_for_link_bindings: this requirement is why decision:explicit-locale-in-links needs two bindings rather than one whose behavior follows its location
non_goals:
  - generating a localized variant from a source image
  - per-locale entries in the transform matrix of policy:asset-transform-matrix, which stays keyed on format and size
```
