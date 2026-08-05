---
id: requirement:page-request-context
type: requirement
title: A Page Must Reach Its Request Context
---
A route under concept:page-tree must be able to read the context of the request rendering it, so a page needing a database pool or an authenticated session keeps the typed entry point instead of dropping to a handler.

```yaml
owner: system:tinybind
status: not raised upstream as of 2026-08-05
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
    fact: routetree validates every Load parameter as a URL-bindable scalar, so a leading context.Context is rejected
    diagnostic: correct about URL inputs, and a context is not one; it arrives from the request rather than from the address
    shape_asked_for: recognize a leading context.Context syntactically, trim it from the input list, and pass the request's context as the first argument
  async_external:
    fact: an external async is excluded from context injection, and unlike live it receives no context through its own call shape either
    upstream_reason: stated as covering async and live together; verified true for live and not reproduced for async
    open: whether the exclusion is deliberate
fourth_gap:
  fact: a page tree whose every route is on the handler rung emits a registry importing the error package it never uses, so the package does not compile
  where: routetree writes the configured error import unconditionally, and the block using it belongs to the decoders of the other two rungs
  why_it_bites_here: the handler rung is the workaround for the three gaps above, so a project that took it for every page meets this
  masked_by: the scaffolded tree, whose template-only root uses the import; the shape appears once somebody deletes that page, which is what a project with an older handler package does
what_this_framework_does_meanwhile:
  rung: a page needing the request context takes the handler rung, func Load(w, r), which generates the registration and owns the response
  cost: the typed rung checks the function's result list against the component's parameter list and the handler rung checks nothing, so the page most needing that check is the one that cannot have it
  documented: requirement:tutorial-page-tree-chapter says why its page takes that rung, rather than presenting it as the ordinary shape
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
