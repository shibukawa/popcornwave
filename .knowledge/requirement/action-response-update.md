---
id: requirement:action-response-update
type: requirement
title: Action Response Update
---
A mutating handler answers the same request with the regions its action changed, so one round trip both performs the action and refreshes the page instead of costing a second fetch that re-derives what the handler already knew.

```yaml
source: decision:update-runtime-convergence
motivation: acting and then re-fetching is two round trips for one gesture, and the second one recomputes state the handler was holding
negotiation:
  signal: the action mode on the shared render header of api:html-update-options
  branch_point: one predicate in the handler, which is what keeps the update path and the ordinary path from drifting apart
  absent: the handler's ordinary response, unchanged; api:api-response JSON for an API endpoint and a redirect for a form handler
  effect: one endpoint serves a non-browser client, a page without the runtime, and an update-capable browser
handler_shape:
  writes: a list of target ids paired with bound components, chosen in Go after the mutation succeeded
  target: the rendered root element must carry the id named, or the region becomes unaddressable after the first update
  count: many regions in one response, because one action commonly changes several
  navigate_instead: a directive replacing the region list when the action changed where the user belongs
  head: from system:tinybind v0.3.2 the response carries each written region's own head contributions, deduplicated across the set, so an action revealing a component the document never held installs its stylesheet before the markup lands
  why_it_matters_here: an action commonly reveals a validation summary or a panel that was not previously rendered, which is exactly the first-appearance case
  entry: the pw response path, so an action writes regions through the same surface api:html-response writes a page, and never constructs a system:tinybind value
status:
  rule: the response carries the handler's real status
  client: applies the regions whatever the status says
  reason: a rejected submission returns 4xx and the regions it carries are the validation errors, which is the point of returning them
  contrast: requirement:reloadable-component-endpoint falls back on a non-2xx, because there a non-2xx means the render failed
  validation_errors: policy:validation-errors decides what the rewritten form region shows, unchanged by the transport
trust:
  new_surface: none, unlike requirement:reloadable-component-endpoint
  reason: the handler authorized the action and picked the components in Go, so no parameter arrives from the caller
  csrf: policy:csrf-protection applies, because the request mutates and carries ambient credentials; the runtime attaches the token header and requirement:module-native-csrf puts one in every unsafe form
  cache: never cacheable
client_state:
  manifest: an action carries none, so it must leave the navigation validators alone
  invalidation: a rewritten region drops its stored validator, or a later navigation could find that boundary unchanged and leave the action's markup in place
  addressing: the framework boundary attribute first, then the author-written element id, since both namespaces are server controlled
relation_to_page_actions:
  endpoint: api:page-action-endpoint is where a page-tree mutation already lives, so this is a response shape for handlers that exist rather than a new address
  handler_ownership: an action handler still owns its whole response, which is exactly what makes the branch expressible
  scriptless: without the runtime the same handler redirects, so post-redirect-get keeps working and requirement:classic-web-acceptance loses nothing
acceptance:
  - a request without the header receives the endpoint's ordinary response, byte for byte
  - one request performs the action and rewrites every region it changed
  - a 4xx action still rewrites the region carrying its errors
  - an action never restates the manifest and never leaves a stale validator behind
  - an ordinary JSON response is never mistaken for an update
  - a form posted with JavaScript disabled reaches the same handler and redirects
  - a region revealed for the first time by an action arrives styled, with no flash
  - two rewritten regions declaring one stylesheet install it once
who_issues_the_request:
  answered: 2026-08-11, as requirement:action-invocation-runtime; the runtime issues it from a gesture on a lowered server action, and api:client-update-api keeps apply and updateHeaders for an application fetch that has no element to hang on
  supersedes: the open question below asking whether a form submission helper belongs on that surface
  why_it_had_to_be_the_runtime: the element already carries a generated address, so leaving the fetch to the application meant every project rewriting one interception, and a form left to the browser was submitted as a GET
open_questions:
  - whether an action may request a navigation-style diff instead of naming regions
```
