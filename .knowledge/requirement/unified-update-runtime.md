---
id: requirement:unified-update-runtime
type: requirement
title: Unified Update Runtime
---
One browser asset, popcornwave-runtime.js, carries every client capability the framework owns, so a document declares one module reference whether it streams boundaries, keeps a live connection, applies a navigation delta, redraws a region, or applies an action response.

```yaml
source: decision:update-runtime-convergence
supersedes_delivery_of: requirement:external-boundary-runtime, whose scope becomes one half of this asset rather than its own file
as_built:
  asset: one served file composed at process start from the module's exported source, this framework's boundary runtime, and a bootstrap that builds the instance
  both_halves_are_files: boundary.js and updateboot.js are real files embedded at build time rather than Go string literals, so a formatter, a linter, and an editor all read them
  no_copy: the module's bytes come from the pinned dependency, so an upgrade that changes them changes this asset and its revision with it
  order_is_load_bearing: the module's half registers the factory and the bootstrap below it builds the instance; the module's own self-instantiation reads document.currentScript, which is null in a module script, so it cannot produce a second instance
  configuration_channel: an inert escaped meta element, because a module script has no way to read its own tag
composition:
  pw_half: the boundary apply function, the parser-path custom elements, the document end marker, truncation reload, and the live reader of api:live-delivery-protocol
  upstream_half: mode negotiation, manifest bookkeeping, delta application, head synchronization, redraw, action apply, link and GET-form interception, history, scroll, focus, form-state reconciliation, and preserved islands
  shared_core: one apply function both halves call, so a boundary lands the same way whichever path delivered it
  header_names: supplied as configuration rather than compiled in, from system:tinybind v0.3.1; the upstream half reads its attribute prefix, header namespace, endpoint prefix, and installed name from one object the server builds
  instantiation: upstream exposes a factory rather than installing a global, so the merged asset constructs one instance under this framework's own name and the two halves share one boundary id space
delivery:
  path: the revision-stamped reserved path of requirement:framework-script-assets, under the name popcornwave-runtime.js
  revision: a digest over the merged bytes, so an upstream upgrade that changes the runtime changes the URL with no constant to bump
  caching: public, max-age one year, immutable, which stays honest because a revision segment never serves different bytes
  form: an ES module referenced with type=module, so it defers by default and never blocks parsing
  no_inline: nothing is inlined on any path, so policy:security-response-headers can enforce script-src self with no nonce
bootstrap:
  tag: contributed by the framework at the render call, per decision:runtime-tag-injection, so no application file has to carry it and no shell edit can remove it
  url_stability: the framework builds the reference from its own revision, so nothing in any template names the URL and an upgrade moves it with no edit anywhere
  runtime_data: the endpoint prefix and the build identity travel as inert escaped head metadata on the same channel, never as inline script
  csrf_token: read from a cookie at the moment a request is issued rather than from the page, per requirement:module-native-csrf, so a rotation reaches an already-open screen
  capability_gating: gated on updates being enabled or the chain declaring an await or live block; decision:automatic-async-render-selection still decides only how to render, and injection is what makes not loading an option at all
  fragment_path: nothing is contributed, since decision:fragment-head-rejection means a fragment response has no head to merge into
client_state_preservation:
  unchanged_boundary: never touched, which is what makes focus, selection, and animation survive
  form_values: reconciled by comparing each control against its own default, so a user's typing survives an update that did not assert a new value, and a changed default wins
  ime: an update is deferred while a composition is active
  preserved_islands: a marked region has its live node moved into the replacement, for a third-party widget, a canvas, or a media element the server does not own
  file_input: not restorable by value at all, so it belongs in a preserved island or outside the region
  known_gap: a GET update cannot express clearing a form back to an unchanged default, because the markup is identical; post-redirect-get clears through an ordinary page load and is outside the rule
failure_behavior:
  rule: every failure path performs the ordinary browser navigation to the same URL, so a user action is never lost
  no_javascript: links and forms work as they always have
  truncation: a streamed document reaching readyState complete with no end marker is reloaded once, with the guard that stops a server truncating every response from producing a reload loop
declaration_order:
  rule: every module-level binding is declared above the first customElements.define, since defining an element upgrades already-parsed ones synchronously inside the call
  guarded_by: a test over the script text, because the failure is a silent load-time throw invisible to every Go-level assertion
acceptance:
  - a strict CSP with no inline allowance applies every boundary, every delta, and every action response
  - one document loads exactly one framework script, whatever capabilities it uses
  - an upstream upgrade changes the served URL without editing the scaffolded shell
  - an unknown revision under the reserved path answers 404 rather than reaching the application
  - a completion whose bytes are split across chunks never destroys its fallback
  - typed text survives an update whose server-rendered default did not change
  - an application removing every script tag from its document shell still applies boundaries and still updates
  - an upstream upgrade that changes the runtime changes the merged asset and its revision, with no copy to reconcile
  - no application template and no served byte carries the dependency's name
open_questions:
  - whether the upstream half is fetched on demand as a capability module, keeping a static page at today's bytes, per the loading design of requirement:framework-script-assets
```
