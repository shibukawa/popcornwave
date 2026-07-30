---
id: concept:page-tree
type: concept
title: Page Tree
---
A page tree is an opt-in directory whose subdirectories are both URL segments and Go packages, so creating a directory with a page template creates a route without a registration call.

```yaml
root:
  declared: data:project-config generate.pages, which is what names it
  scaffolded_as: pages, a default rather than a fixed name
  served_by: the discovered router, which is what the project calls this half of decision:dual-router-coexistence
  coexists_with: the handlers tree of flow:handler-registration, per decision:dual-router-coexistence
route: a directory holding page.pw.html, anywhere below the root
reserved_names:
  page.pw.html: the page component
  layout.pw.html: an ancestor wrapper for every page below it
  page.go: optional Go beside the page, holding Load and the api:page-action-endpoint handlers
  document.pw.html: discovered by system:tinybind and not applied; decision:implicit-document-shell owns the shell
naming: rule:page-directory-naming
rungs:
  reason: one file is a page, and a signature is what raises it
  template_only:
    files: page.pw.html
    handler: fully generated
    data: the template's own external calls, per api:typed-external-function
  typed:
    files: page.pw.html and page.go with func Load(id string, page int) (User, error)
    handler: generated; it decodes the URL, calls Load, and renders the results
    contract: the page component's parameter list is Load's result list, checked by count, order, and type
  handler:
    files: page.pw.html and page.go with func Load(w http.ResponseWriter, r *http.Request)
    handler: registration only; the response is the application's
    chain: a handler-rung Load cannot call an ancestor composer, so it builds the chain itself with api:render-html-chain
  mismatch: a Load matching neither shape fails generation, naming the signature it has and the two it could have
inputs:
  order: leading parameters are the dynamic segments in route order; the rest are query parameters keyed by parameter name
  read_from: the page component's parameter list without page.go, and Load's parameter list with it
  types: scalars only, because a URL carries no object
  catch_all: bound as a string
  optional_query:
    spelling: a trailing question mark on the declared type, which binds a pointer left nil when the key is absent or its value is empty
    since: system:tinybind v0.2.5, before which an absent query value was indistinguishable from a zero one
    path_segments: rejected, with a distinct reason for a single segment and for a catch-all, because a matched route always has them
layouts:
  scope: every ancestor layout.pw.html wraps the page, outermost first
  declaration: the layout must declare children as html, because that shape is what makes the template compiler emit a wrapper binder
  visibility: a layout reads only the dynamic segments at or above its own directory, so a wrapper stays reusable across the segments below it
  missing_declaration: reported by discovery rather than left to the Go compiler
naming_note: the entry point is Load rather than Page because the template compiler already emits Page in the same package
```
