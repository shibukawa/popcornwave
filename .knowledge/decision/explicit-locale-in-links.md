---
id: decision:explicit-locale-in-links
type: decision
title: The Locale Is Written Into Links
---
An author writes the locale into a URL through a declared binding, and the compiler rewrites nothing it was not pointed at.

```yaml
status: proposed downstream; the upstream mechanism is delivered in system:tinybind v0.5.13
source: user discussion 2026-08-16
spelling:
  route_link: "<a href=\"/{lang}/about\">"
  dynamic_segments: "<a href=\"/{lang}/products/{id}\">, which is ordinary interpolation"
  localized_asset: "<img src=\"/assets/{langtag}/hero.png\">"
  document_language: "<html lang=\"{langtag}\">"
  unlocalized: "<img src=\"/assets/logo.png\">, written with neither binding"
bindings: data:locale-bindings, which carries the registered set and their kinds
why_two_bindings_and_not_one:
  proposal: one name whose behavior follows its location, since the lang attribute clearly wants a tag and an href clearly wants a segment
  counterexample: "<img src=\"/assets/{lang}/hero.png\"> is a url attribute and still wants the tag, because requirement:localized-assets addresses a localized asset by URL in every mode"
  not_an_edge_case: under path mode with prefix_default false, the default locale's asset path collapses to /assets/hero.png, which may exist as the unlocalized file, so the wrong image is served with no error
  why_location_cannot_decide: separating a route link from an asset link would need to classify a path as route or asset, and no attribute does that; an anchor may reach a document and an image source may reach a route
  same_wall: that classification is what made automatic rewriting impossible, so deciding by context reintroduces it
  what_the_split_actually_is: whether the value can be empty, which is the binding kind and also decides whether the collapse fires
  a_third_appeared: data:locale-bindings records the typed message binding, which is neither of these and is never written into markup
collapse:
  cases:
    - "/{lang}/about with ja gives /ja/about, with empty gives /about"
    - "/{lang}/ with ja gives /ja/, with empty gives /"
    - "/{lang} with ja gives /ja, with empty gives /"
  third_case_is_the_one_a_naive_rule_gets_wrong: a trailing segment collapses to the root rather than to the empty string, so the separator is surrendered only where something follows
  where: generation, on the literal pattern the compiler can see
  not_at_runtime:
    would_break: a protocol-relative URL an author wrote deliberately
    would_reach: a URL the framework did not compose
  scoped_to_url_contexts: policy:template-escaping already identifies them, and collapsing inside prose would be wrong
  what_it_removes: two checks an earlier design needed, since /{lang} is now both the correct spelling and safe when empty
  why_the_collapse_is_not_the_rewriting_this_replaces: it is local to a marker the author wrote and has one correct outcome, with no path classified and nothing inferred
the_segment_value_is_attacker_influenced:
  fact: a locale is resolved from a path prefix or from Accept-Language, so it originates in request input even though the binding is framework-supplied
  upstream_response: percent-encode everything outside the unreserved set, and encode a dot segment rather than pass it through
  what_that_closes: path traversal, a leading double slash, an embedded query or fragment, and a smuggled scheme, none of which can compose a path the template did not describe
  what_it_does_not_close: which strings become a locale tag at all, which is this project's obligation
  downstream_rule: a resolved locale is always one of the declared set, never a tag echoed from the request, per rule:locale-prefix-checks
  why_that_matters_even_with_encoding: an encoded arbitrary tag still reaches the origin server as a distinct URL, so echoing one turns every unmatched Accept-Language into a cache entry
correction_to_the_original_justification:
  claimed: "an empty q in href=\"/search/{q}\" renders /search/ today, so a rule keyed on emptiness would change existing behavior"
  actual: that template does not compile at all, because the url attribute gate refuses a plain string in any interpolated part
  effect_on_the_decision: the compatibility argument is void, and the conclusion is unchanged; keying on the binding kind keeps a future url-typed interpolation from acquiring a collapse it never asked for
rejected_alternative:
  automatic_rewriting:
    shape: the compiler rewrites a root-relative href against the route table
    why_not: it must classify a path as a route or an asset to know whether to rewrite, and requirement:localized-assets makes both shapes identical
    what_survived: the route table is still consulted, for the diagnostics of rule:locale-prefix-checks rather than for a transformation
    difference_in_failure_mode: a wrong transformation ships silently, a missed diagnostic is a warning
  separator_inside_the_binding:
    shape: "{lang}/about with lang carrying its own leading slash"
    why_not: correct, but /{lang} reads better and the collapse removes the reason to hide the slash
dynamic_urls:
  not_rewritten: "href={u} carries no visible pattern"
  answer: api:locale-accessors LocalePath, which is also what Go-side URL construction uses
```
