---
id: requirement:external-boundary-runtime
type: requirement
title: External Boundary Runtime Script
---
Popcorn Wave ships the client runtime that applies boundary completions, declared once in the document shell as a module reference and never as inline script.

```yaml
ownership: whole, since system:tinybind v0.1.20 emits placeholders only and defines no custom element
scope:
  - apply a completion to its placeholder, for both api:html-boundary-protocol envelopes
  - replace the document body for decision:unhandled-boundary-escalation, then ignore anything that follows
  - remove the placeholder, template, and marker on apply, so nothing accumulates
  - namespace inserted placeholder ids per response on the fetch path
  - register the parser-path custom element as a thin adapter over the apply function
  - preserve the framing invariants the module no longer enforces
live_scope:
  status: implemented with requirement:live-html-rendering
  added:
    - read the api:live-delivery-protocol document marker and open a live connection only when it says live work remains
    - read the record stream, applying each delivery through the same apply function, so the live path adds a reader rather than a second runtime
    - reconnect, back off, and stop, per requirement:live-connection-recovery
    - abort the connection before applying a same-document navigation
  unchanged: the module reference in the scaffolded document shell, so a live page loads no extra asset and policy:security-response-headers still needs no nonce
  cost: the reader ships to every page that loads the runtime, whether or not that page has a live boundary
  open: whether the live half is a separate module fetched on demand, which would keep a static page's first visit at today's bytes
declaration_order:
  rule: every module-level binding is declared above the first customElements.define
  why: defining an element upgrades the ones the parser already inserted, synchronously, inside the define call, so a callback reading a binding declared further down reads it before its initializer has run and throws
  why_it_is_not_a_race: the document marker is always already in the DOM by the time its element is defined, so this is the ordinary path
  how_it_failed: the marker's callback threw during load, silently, leaving a page that applied its boundaries and then never updated; found by running the example in a browser, not by any test
  guarded_by: a test over the script text, since the failure is invisible to every Go-level assertion
placement:
  superseded_by: decision:runtime-tag-injection, which moves the reference out of the scaffolded shell and into a framework contribution at the render call
  was: the api:cli-init scaffolded templates/document.pw.html, so every page loaded it
  why_it_was_chosen: an always-present scaffolded tag removed the need for a head-injection hook system:tinybind did not yet offer
  branch_independence: unchanged in effect; decision:automatic-async-render-selection still decides only how to render, never what to load
  cost_now_avoidable: a page with no dynamic behavior and updates disabled can load nothing, which a scaffolded tag could not express
url_stability:
  problem: the revision of requirement:framework-script-assets moves on a dependency upgrade, but a scaffolded template is written once and then owned by the author
  not_a_parameter: generated registration binds the document with an empty parameter struct at package init, before configuration is parsed, so the URL cannot arrive as a bound parameter
  was_resolved_by: "a declared `external RuntimeScriptURL(): url` the scaffolded templates package implemented over the framework accessor"
  no_longer_needed: injection means no template names the URL at all, so the indirection has nothing left to protect; the declaration is removed from the scaffold rather than kept as a second way in
failure_mode:
  symptom: boundaries never apply and every fallback stays, with no console error and no failed request
  cause: the shell reference is missing or was removed, and nothing else supplies a runtime
  why_it_was_reachable: the reference lived in a file the author owns, edits, and may rewrite, and a scaffold test says nothing about a project a month later
  closed_by: decision:runtime-tag-injection, since the framework contributes the reference itself and no shell edit can drop it
  residual: a shell declaring no head element has nowhere for any contribution to land, which rule:route-and-template-checks reports rather than leaving silent
  unchanged_guard: the reserved path is served unconditionally, so no configuration combination can leave the reference unresolvable
script_loading:
  choice: module script, which defers by default
  correctness: a marker element parsed before the definition upgrades when customElements.define runs, and upgrades happen in document order, so no completion is lost
  cost: boundaries that settle during parsing stay as fallback until the module executes, which inline delivery avoided
  rejected: a classic src script without defer blocks parsing until the fetch completes
delivery: requirement:framework-script-assets, of which this is the first consumer
merged_at_v0_3_0:
  what: this script stops being its own file and becomes the async and live half of requirement:unified-update-runtime, served as popcornwave-runtime.js
  why: system:tinybind v0.3.0 ships a runtime covering the same boundaries plus navigation, redraw, and action apply, and two runtimes on one document would mean two boundary id spaces and two build identities
  unchanged: the reserved path, the content-derived revision, the single shell reference, the declared external returning the URL, and every acceptance below
  decided_by: decision:update-runtime-convergence
benefits:
  - policy:security-response-headers can enforce script-src 'self' with no nonce, hash, or unsafe-inline
  - one cached file covers every page and every deployment of one dependency set
acceptance:
  - a strict CSP with no inline allowance still applies every boundary
  - a completion whose bytes are split across chunks never destroys its fallback
  - a dependency upgrade changes the served URL with no template edit anywhere
  - an unknown revision under the reserved path answers 404 rather than reaching the application
  - the applied document retains no placeholder, template, or marker element; it retains the comment brackets api:html-boundary-protocol keeps as a live delivery's address
  - a document cut off mid-stream is detected from the missing terminal marker and reloaded once, with the guard that stops a server truncating every response from producing a reload loop
  - an identical delivery for a boundary already showing that content changes no node, so focus, selection, and animation inside it survive a reconnect
```
