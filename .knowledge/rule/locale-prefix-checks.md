---
id: rule:locale-prefix-checks
type: rule
title: Locale Prefix and Link Checks
---
The route table and the asset manifest are read to diagnose a locale mistake, never to repair one, because a missed diagnostic is a warning and a wrong repair ships silently.

```yaml
route_shape:
  - a concept:page-tree top-level directory whose name matches a declared locale tag is an error, since the prefix position is always read as a locale
  - a request whose prefix position holds an undeclared tag is 404, never reinterpreted as an ordinary path segment
  - a locale prefix arriving at a cookie or header mode route is 404, because that route has no prefixed form
  - the default locale's own prefix under prefix_default false is redirected permanently to the unprefixed path
link_shape:
  - a literal href in a path mode template that resolves to a path mode route and carries no locale binding is a warning naming the missing binding
  - a literal href or src resolving to neither a declared route nor a manifest entry is a warning, which is a dead-link report independent of this feature
  - the second check runs in single-locale projects too, since it needs no locale to be useful
why_diagnostics_and_not_rewriting:
  fact: decision:explicit-locale-in-links rejected rewriting because a path cannot be classified as route or asset once requirement:localized-assets exists
  what_survives: the same tables answer a question about a link the author already wrote
  failure_mode: a missing binding produces a page in the wrong language rather than a 404, which is why a test suite does not find it and a warning is worth its noise
resolved_locale_is_never_echoed:
  rule: a resolved locale is always a member of the declared set, never a tag taken from request input
  why: data:locale-bindings writes the value into a path, and system:tinybind percent-encodes it but cannot know which strings are locales
  what_an_echo_would_cost: every unmatched Accept-Language becomes a distinct URL at the origin, which is an unbounded cache surface rather than a rendering fault
  redraw_path: the component redraw endpoint derives the locale from the request rather than accepting it as a passed input, since a client-supplied one would let a reader choose which cached representation they receive
catalog_shape:
  - a reference to an ID no catalog defines is an error
  - an ID defined and referenced nowhere is a warning, since removal is a human judgement
  - a hole named in a translation and unbound at its reference is an error, and a hole bound and named by no translation is an error, per policy:message-rich-text
  - an extraction mark left in a shipped template is a warning, because an unknown attribute passes through and a stray one reaches the rendered output
  - a translation whose placeholder set differs from the declaration is an error in every configuration
  - a locale missing a plural category its rules require is an error
  - a locale missing a message follows the severity of data:i18n-config i18n.missing
  - a source-locale text differing from the snapshot recorded at assignment marks every other locale stale, per decision:message-id-assignment
asset_shape:
  - a locale-named directory under an asset root is a localized set, and a member absent for a declared locale is reported, per requirement:localized-assets
placement: beside rule:route-and-template-checks, which owns the non-locale half of the same tables
```
