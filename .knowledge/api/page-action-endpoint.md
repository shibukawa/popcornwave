---
id: api:page-action-endpoint
type: api
title: Page Action Endpoint
---
A page template names an exported Go handler in its own route package and generation supplies the address, so a mutation reachable from a page is a resolved symbol rather than a hand-written URL string.

```yaml
scope: the mutation half of concept:page-tree; the typed contract of api:server-action is a separate rung that does not exist yet
template: 'a server-action attribute naming the handler lowers to data-pw-action carrying the endpoint, and every other attribute on the element is left untouched'
handler:
  shape: an ordinary func(http.ResponseWriter, http.Request) that owns its whole response
  generated_around_it: nothing but the registration
  testable: with net/http/httptest, without registering anything
address:
  form: /_action/<hash>/<HandlerName>
  hash: the leading 12 hex digits of a digest over the declaring directory and the handler name
  declaring_directory: not the serving route path, because a layout compiles once and renders under many paths, and hashing the path would give one handler an address per page
  stability: no build salt, so an unchanged project regenerates the same address and a page left open across a deploy posts somewhere the server still knows
  readable_half: the handler name rides along, so a network trace names the Go function that ran
prefix:
  value: /_action
  not_under_pw: requirement:framework-script-assets answers 404 for an unknown path below its reserved prefix ahead of application routing, so an action mounted there would never reach the mux
  safe_by_construction: rule:page-directory-naming ignores a directory starting with underscore, so no page route can produce this prefix
  configured_prefix: generation rejects a prefix an existing route already occupies rather than leaving it to a ServeMux panic
reachable_surface:
  rule: every exported handler-shaped function in a route package gets an endpoint, whether or not a template mentions it
  bounded_by: a route package is imported only by the generated registry, so its exported symbols are that route's surface rather than a general API
  opt_out: lowercase the function, since generated code in another package cannot reach an unexported symbol
  excluded: Load, which is the page entry point of concept:page-tree
  inspectable: the data:page-route-table Actions list enumerates every endpoint
authorization:
  - an address hides structure and grants nothing, so each handler authenticates and authorizes its own caller
  - policy:authenticated-path-protection patterns apply to the prefix like any other path
  - policy:csrf-protection must cover the prefix once that middleware exists; today nothing does, so an action authorizes its own caller and nothing else stands in front of it
request_binding:
  available: api:request-binding works inside an action, so a handler reads its input with pw.Parse like any other
  mechanism: binder generation reads the Bind call sites of the package it analyzes and never consults a registration, so what was missing was only a generation run over the route packages; flow:page-route-generation adds one
  corrected_cause: the earlier reading, that binder discovery was driven from application-written registrations, was wrong; recording it matters because it is what made this look like an upstream redesign rather than one more package list
openapi:
  absent: no document entry, by design, since an action is one page's implementation detail
  not_a_side_effect: once generation reaches the route packages, the generated registry is itself a registration site, so the exclusion is something flow:page-route-generation has to keep rather than something the package boundary provides
  scriptless_forms: a form server-action still lowers to an attribute and needs a runtime to intercept it, so the post-to-self and 303 shape is a later rung; until it lands, adopting actions costs the no-runtime property requirement:classic-web-acceptance asks for
  client_runtime: the attribute needs the action capability module of requirement:framework-script-assets, which the boundary runtime does not yet contain
outside_a_page_tree:
  possible: system:tinybind v0.2.5 accepts an action resolver, so a framework can answer a name from its own route table
  use: a handlers-tree template could carry server-action once pw supplies a resolver over its rule:static-route-discovery results
  not_in_this_delivery: the page tree is where the addresses already exist, so the resolver is a follow-up rather than a prerequisite
rationale: a URL is a string never checked against the handler it names, while a symbol must resolve, so renaming the Go function fails generation at the template that referenced it
```
