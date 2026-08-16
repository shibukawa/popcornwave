---
id: requirement:page-request-context
type: requirement
title: A Page Must Reach Its Request Context
---
A route under concept:page-tree must be able to read the context of the request rendering it, so a page needing a database pool or an authenticated session keeps the typed entry point instead of dropping to a handler.

```yaml
owner: system:tinybind
status: answered 2026-08-14, by a different declaration than the one this asked about
answered_rather_than_delivered:
  what_this_asked_for: a leading context.Context on a typed Load, which shipped in v0.5.8 and was verified here
  what_happened_next: system:tinybind v0.5.13 removed the typed Load itself, and the context parameter with it
  why_the_requirement_is_still_met: a page's data now comes from an external declared in its template, and a route-package external declaring a leading context receives the request's
  so: the subject moved from the page entry point to the loader, and the thing this existed to make possible — a page reading the database pool or the signed-in reader — is expressible again
  taken_as: requirement:template-bound-page-loader, which carries the shape and the migration
  what_is_not_recovered: nothing this requirement asked for; the typed check it valued moved to the component's own parameter list rather than disappearing
what_is_unverified: the context externals gap looks closed too, since routetree now threads ContextExternals into the tree compile, and the fourth gap's error import is still written unconditionally; neither was exercised, so neither is reported as an answer
request: docs/tinybind-go-page-context-request.md
priority: should
evidence:
  surveyed: 2026-08-05 against v0.3.5, by generating and building each shape
why_it_matters_here:
  placement: this framework puts the database handle and the authenticated session on the request context, per api:request-context-accessors
  scope: the module needs to pass the context through; what is on it is this framework's business and not the module's
  ordinary_page: a page listing the signed-in reader's own records is the most common page a website has, and it is the case that cannot be expressed
three_gaps:
  page_context_externals:
    fact: routetree compiles a page template without ContextExternals, so a synchronous external in a route package receives no context
    inconsistent_with: the same declaration in a templates directory, which does receive one; generator templateArtifacts computes the set and routetree compileTemplate does not
    smallest: this is the one gap that is an oversight rather than a design question, and closing it alone would give a page a way to reach the context with no new concept
  typed_load:
    closed: v0.5.8, verified 2026-08-13 by declaring one in the page tree fixture and reading what generation emitted
    as_shipped: exactly the shape asked for — takesLeadingContext recognizes it syntactically, trims it from the input list, and the registration calls Load with the request's context first
    proved_by: the fixture's Load, whose generated call site reads id_.Load(r.Context(), route.ID, route.Page)
    was: routetree validated every Load parameter as a URL-bindable scalar, so a leading context.Context was rejected
    diagnostic_unchanged: a context anywhere but the first position still gets the ordinary not-a-URL-value error, which is the right answer there
  async_external:
    fact: an external async is excluded from context injection, and unlike live it receives no context through its own call shape either
    upstream_reason: stated as covering async and live together; verified true for live and not reproduced for async
    open: whether the exclusion is deliberate
fourth_gap:
  fact: a page tree whose every route is on the handler rung emits a registry importing the error package it never uses, so the package does not compile
  where: routetree writes the configured error import unconditionally, and the block using it belongs to the decoders of the other two rungs
  why_it_bites_here: the handler rung is the workaround for the three gaps above, so a project that took it for every page meets this
  masked_by: the scaffolded tree, whose template-only root uses the import; the shape appears once somebody deletes that page, which is what a project with an older handler package does
what_this_framework_did_meanwhile:
  rung: a page needing the request context took the handler rung, func Load(w, r), which generates the registration and owns the response
  cost: the typed rung checks the function's result list against the component's parameter list and the handler rung checks nothing, so the page most needing that check was the one that could not have it
  no_longer_forced: with the typed half shipped, a page reading the pool or the session keeps the checked rung; the handler rung stays the escape for a page that owns its response rather than the workaround for this
  documentation_follows: requirement:tutorial-page-tree-chapter explains why its page takes the handler rung, and the reason it gives is this gap
acceptance:
  - a typed Load declaring a leading context.Context generates and receives the request's context
  - a synchronous external in a route package receives a context on the same terms as one in a templates package
  - every parameter after the context stays a URL-bindable scalar, with the current diagnostic unchanged
non_goals:
  - an http.Request in a typed Load, which would put the transport into a signature whose point is that it has none
  - the module knowing what this framework stores on the context
  - replacing the handler rung, which stays the right escape for a page that owns its response
related:
  - requirement:discovered-page-routing
  - api:page-render-runtime
  - api:typed-external-function
```
