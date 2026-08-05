---
id: flow:partial-refresh
type: flow
title: Boundary Partial Refresh
---
An interaction refreshes only the regions it affected, through whichever of the three update paths owns the inputs that changed.

```yaml
superseded_sketch: one POST patch endpoint deriving affected boundaries from data:ui-dependency-graph; system:tinybind v0.3.0 answers the same problem with three narrower paths and a client-held manifest, so the endpoint and the server-side graph walk are not built
paths:
  navigation:
    owns: inputs the server derives, such as a search parameter or a route change
    request: the page's own URL and method, carrying the render header, the manifest hint, and the build identity
    server: render the same chain, compare each boundary frame validator against what the browser sent, and send the outermost changed boundaries only
    detail: requirement:navigation-delta-rendering
  redraw:
    owns: inputs the browser holds, for a region whose state should not reach a shareable URL
    request: a GET to the reserved redraw path, naming the component kind, the instance id, and every declared parameter
    server: run that component alone, with no page execution and nothing to compare
    detail: requirement:reloadable-component-endpoint
  action:
    owns: a mutation, where the handler already knows what changed
    request: the application's own fetch, carrying the render header in action mode and the policy:csrf-protection token
    server: perform the mutation, then write the regions it changed with the handler's real status
    detail: requirement:action-response-update
client:
  apply: one apply function for every path, per api:html-boundary-protocol
  supersede: an older in-flight request for the same target is aborted, and a superseded response is discarded unapplied
  preserve: focus, selection, IME composition, form values, and preserved islands survive, per requirement:unified-update-runtime
  fall_back: any failure performs the ordinary browser navigation, so a user action is never lost
  drive: api:client-update-api, or an intercepted link or GET form on the same path
identity_and_freshness:
  boundary: data:component-boundary, whose instance identity and output validator are written by generation onto the boundary root element
  invalidation: the browser holds the validators and the server compares them, so nothing is kept per session and a restart loses nothing
  build: a page rendered by another build is answered with a complete document rather than a delta
choosing:
  rule: state that can live in the URL belongs there, so navigation handles it and the page stays shareable and bookmarkable
  otherwise: the component fetches its own data and is registered reloadable
  no_middle: patching one parameter of a re-run handler cannot reach the data fetch that produced the component's other inputs, so there is no third mode
unmanaged_alternative: requirement:html-fragment-rendering, where the application owns the route and an external swap library owns application, with no negotiation, no boundary identity, and no ordering guarantee
```
