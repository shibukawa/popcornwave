---
id: decision:update-runtime-convergence
type: decision
title: Update Runtime Convergence
---
system:tinybind v0.3.0 ships htmlupdate, a complete update transport with its own browser runtime, header namespace, and endpoint prefix, duplicating what Popcorn Wave already owns; adopt it as the transport and merge the two browser runtimes into one asset under the existing Popcorn Wave names rather than retiring either side.

```yaml
source: user convergence decision 2026-08-01
what_upstream_now_ships:
  package: tinybind-go/htmlupdate, the net/http half htmlbind deliberately excludes so generated plans stay TinyGo-safe
  modes: navigation delta, component redraw, action response, streamed delta records, live reconnect
  runtime: one embedded runtime.js served from its own versioned path
  names: from v0.3.1 every name is configuration reaching both halves as one object, so nothing this framework calls carries the dependency's brand
  serving: the module serves its own asset by default and stops when the caller declares ownership
what_pw_already_owned:
  runtime: the boundary runtime of requirement:external-boundary-runtime, serving async completion apply, live delivery, and truncation reload
  header: the live mode token on Pw-Response-Mode
  path: the reserved prefix of requirement:framework-script-assets
  framing: the envelopes of api:html-boundary-protocol
overlap: async streaming, live delivery, and reconnect exist on both sides; navigation delta, redraw, action response, form reconciliation, and preserved islands exist only upstream
decision:
  transport: htmlupdate is the implementation, wrapped by api:html-update-options; pw writes no delta encoder, no manifest codec, and no redraw router of its own
  header_namespace: one Pw prefix, so the render header is Pw-Render and the hint headers are Pw-Manifest and Pw-Build
  one_header_many_modes: the live token joins the same header as a mode value rather than keeping a header of its own, because the modes are mutually exclusive readings of one question
  runtime_asset: the two scripts merge into one popcornwave-runtime.js, served by requirement:unified-update-runtime from the reserved prefix
  endpoints: htmlupdate mounts under the pw reserved prefix, so requirement:framework-script-assets keeps one routing, caching, and access rule
why_coexistence_works:
  negotiate_is_strict: htmlupdate resolves anything that is not its own mode token to a complete document, so an unrecognized value is inert rather than an error
  consequence: a live token on the shared header reaches htmlupdate as no update request at all
  ordering_rule: pw must test its own modes before delegating, or a live request would be answered as a document; this is the one thing the shared header costs
  version_suffix: the live token gains the ;v=N form the other modes carry, so one parser reads the header
  token_shared_with_a_planned_mode: the module publishes 'live;v=N' in its guide for its own live reconnect, but parses no live token and carries live deliveries on the navigation mode instead, so the two describe the same intent with different bodies; requirement:tinybind-update-composition-seams carries the convergence
retired:
  - Pw-Response-Mode as a distinct header, replaced by a mode value on the shared one
  - the separate boundary runtime file, absorbed rather than deleted
kept:
  - every envelope of api:html-boundary-protocol, since the async and live paths are unchanged on the wire
  - api:live-delivery-protocol records and the document end marker
  - the reserved-path and immutable-revision rules of requirement:framework-script-assets
upstream_request:
  raised: requirement:tinybind-runtime-ownership, ten findings against v0.3.0
  answered: v0.3.1, every item accepted, which removes every workaround this decision had allowed for
  effect_here: the merge becomes composition of an exported value instead of vendoring a copy, and every name becomes configuration instead of a rebuild
resolved_asks:
  runtime_source: exported as bytes, as an asset with a digest and media type, and as a configuration the server and browser share; serving is switchable, so this framework serves the merged asset alone
  protocol_names: the runtime reads the attribute prefix, header namespace, endpoint prefix, and installed name from that configuration, so the Pw namespace needs no rebuilt runtime
  author_attributes: the preserve and ignore markers follow the same prefix, so no application template writes the dependency's name
  boundary_prefix: one render option names the placeholder element and the boundary ids together, so the generated attributes and the rendered markup finally agree
  mount: takes a one-method router that api:serve-mux already satisfies, so the endpoints install with one call
  errors: a typed failure reaches a callback, so a redraw failure is visible to api:error-renderer and the request-scoped logger
  registration_and_bounds: registration returns an error, options validate together, and the query bound and stream media type are configurable
how_pw_holds_its_own_half:
  today: a Go const holding the script as a raw string literal, with tests asserting over that string
  cost: the JavaScript gets no editor tooling, no formatter, and no linter, and a merged asset several times its size makes that worse
  change: hold it as a real .js file embedded at build time, so both halves of the merge are files before they are bytes
script_tag_ownership: pw emits its own tag, because injection per decision:runtime-tag-injection is what keeps the reference out of an author-owned file; upstream returns an empty tag once the caller owns the runtime, so the two do not collide
rejected:
  keep_pw_runtime_only:
    what: implement navigation delta, redraw, and action apply on the htmlbind boundary and validator primitives
    why_not: it rewrites form-state reconciliation, preserved islands, history, scroll, and focus handling that upstream already ships and tests
  adopt_upstream_names:
    what: serve under X-Tinybind and /_tb
    why_not: the reserved prefix, the immutable revision URL, and the framework-owned asset rule are Popcorn Wave contracts a dependency name must not leak into
  two_runtimes_on_one_page:
    why_not: two boundary id spaces, two script tags, and two build identities on one document, with nothing deciding which one owns a given region
sequencing:
  - api:html-update-options and requirement:unified-update-runtime, since every other item loads through them
  - requirement:navigation-delta-rendering, which needs no generation change
  - requirement:reloadable-component-endpoint, which needs the reloadable modifier and a registration site
  - requirement:action-response-update, which needs only a branch in an application handler
  - requirement:module-native-csrf, whose token supply and middleware are this framework's remaining half
delivery_of_the_runtime_itself: decision:runtime-tag-injection, which is a prerequisite of the first item rather than a step after it, because a merged asset no page reliably loads is worth nothing
not_sequenced_here: requirement:component-asset-extraction, which concerns application component assets and shares nothing with this convergence but the word script
unblocks: flow:partial-refresh, the rung decision:web-runtime-delivery-order placed after flow:initial-streaming-render
```
