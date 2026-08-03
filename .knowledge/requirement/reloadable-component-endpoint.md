---
id: requirement:reloadable-component-endpoint
type: requirement
title: Reloadable Component Endpoint
---
A component declared reloadable is published as a GET endpoint under the reserved prefix, so browser code can re-render one region by naming its instance and supplying its inputs, with no page execution and no manifest.

```yaml
source: decision:update-runtime-convergence
different_question: requirement:navigation-delta-rendering answers what changed on this page; this answers render this one region again with these values
declaration:
  syntax: a reloadable modifier after export on the component declaration in a .pw.html source
  generated: a typed query decoder, a kind identity, and a registration value
  id_parameter: a required string id the author writes at the call site, filled from the path on a redraw
  diagnostics_are_errors:
    - the component must be exported, single rooted, and not the document shell
    - every parameter other than id must be a type a query string carries deterministically, so a record, a slice, and html are refused
    - unlike an automatic boundary these fail generation, because the author asked for the endpoint
kind:
  value: the component name plus a hash of its parameters and compiled markup
  purpose: version, so editing the template changes the URL and a page loaded before a deploy gets a 404 and reloads rather than rendering under changed semantics
  does_not_cover: the package, so two templates identical in name, parameters, and markup collide
  guard: registering one kind twice fails at startup rather than overwriting, since identical plan text still resolves its external calls per package; it returns an error rather than panicking, so a generated registry reports it as a startup diagnostic
  on_the_element: every render writes the kind beside the instance id, so a region stays redrawable after a redraw replaced it
registration:
  explicit: being exported and single rooted is not enough, because publishing an endpoint must be a deliberate act
  where: the generated api:page-registry Register, which is already the one symbol an application touches to install a page tree
  application_visible: the registration value exists in Go, so a reviewer reads the published set from source rather than from templates
  outside_a_page_tree: a registered-router project installs the value itself, since it owns its own mux
request:
  shape: 'GET <reserved prefix>/redraw/<kind>/<instance>?<declared parameters>'
  build_check: a request from another build is refused rather than answered, because a kind is stable across builds and cannot say whether the page asking is current
  bounds: a configured maximum query length, since a GET carries every argument
  decoding: a missing, undecodable, or repeated parameter is an error, never a zero value
response:
  body: that component's subtree as one root element carrying the same id
  headers: private and no-cache, the served mode echoed on the render header, and a keyed ETag over the rendered bytes
  conditional: an unchanged region costs a 304, which is what makes a polled redraw cheap without a manifest; the ETag is keyed for the same reason the validators are
  privacy: private rather than public, because the URL alone identifies a usually per-user response
  failure: an unknown kind is 404, a stale build is 409, an oversized query is 414, a bad argument is 400, and each falls back to a complete page load on the client
  failure_visibility: every refusal reaches this framework through the hook of api:html-update-options, so it is logged and rendered like any other failure rather than written as plain text by the module
  head: from system:tinybind v0.3.3 both halves are covered without changing the body, which stays a bare subtree so what curl sees is unchanged
  guarantee_side: the registry reports the head and assets of every published component, so the document shell installs them once at startup and covers every redraw of that deployment
  report_side: a redraw of a component that contributes head announces those tags on a response header, and the runtime installs what is missing before replacing the region
  bound: the head one component may carry is checked when it is registered rather than per request, because a component's head is a static declaration and an oversized one is a startup fact
authorization:
  new_surface: yes, and it is the whole cost of this capability
  rule: a registered component receives whatever the caller sends, where a component argument used to be a value the page had already authenticated and authorized
  safe: a component that only formats values handed to it
  unsafe_without_a_check: one that loads a record by identifier, which must verify ownership or visibility itself exactly as a handler does
  review_point: registration; rule:route-and-template-checks reports the published set so a project can review it, and api:cli-doctor names it
  path_protection: policy:authenticated-path-protection patterns apply to the prefix like any other path
  csrf: a redraw is a side-effect-free GET, so policy:csrf-protection does not gate it; origin defence still applies to a credentialed read
idempotence:
  rule: a redraw must be repeatable with no observable effect, because it is retried on supersession and may be answered from a cache
  privacy: per-user output is private, since the URL alone identifies the response
guidance:
  prefer_navigation: state that can live in the URL belongs there, where requirement:navigation-delta-rendering already handles it and the user gets a shareable, bookmarkable page
  earns_its_keep: widget-local state that should not appear in a shareable URL, or a region whose inputs the browser genuinely owns
  no_third_mode: re-running the handler with one patched parameter cannot reach the data fetch that produced the component's other inputs, so a patched sort order never reaches the query
acceptance:
  - a registered component renders from a URL with no page execution
  - an unregistered component has no endpoint, even when it is exported and single rooted
  - a template edit changes the endpoint URL and a page loaded before the deploy falls back to a full load
  - a second redraw of the same region succeeds, because the replacement carries the kind again
  - a component loading a record by identifier is reported by rule:route-and-template-checks as a registration to review
  - registering two components that hash to one kind fails startup validation with a diagnostic naming both, rather than aborting the process
  - a redraw whose region did not change answers 304
  - a refused redraw appears in the request log with its cause, and version skew is distinguishable from a decoding failure
```
