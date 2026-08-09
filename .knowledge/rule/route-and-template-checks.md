---
id: rule:route-and-template-checks
type: rule
title: Route and Template Checks
---
The PW02xx data:diagnostic-check entries: what data:route-table and the template sources say about paths that collide, paths nothing serves, and pages nothing reaches.

```yaml
routes:
  duplicate-pattern:
    trigger: two data:route-table entries registering the same pattern
    severity: error
    reason: api:serve-mux panics at registration, so this is a startup crash found without starting
    preference: api:cli-generate should reject it at generation time where the analysis can see both; doctor still lists it, because a project that cannot generate is exactly when doctor runs
  framework-mount-collision:
    trigger: an application pattern equal to, or shadowing, an enabled framework mount in data:route-table
    severity: error
    evidence: names the configuration key that enables the mount, so moving either side is a choice the reader can make
    reference: policy:operational-endpoints and requirement:public-asset-delivery
  unreachable-route:
    trigger: a pattern that no request can reach, because a mount prefix and the pattern disagree
    severity: warning
  unresolved-registration:
    trigger: a data:route-table unresolved entry
    severity: note
  route-table-diverges-across-backends:
    trigger: the data:route-table built for one backend build configuration differs from the one built for another
    severity: error
    reason: a handler in a file excluded by a build tag takes its registration with it, so the route is absent rather than broken, and nothing else in the toolchain reports a route that quietly stopped existing
    also_covers: a pattern the decision:transport-source-transform absorption layer cannot express identically, per the api:serve-mux cannot_absorb list, which is a difference in meaning rather than in presence
    scope: only when a project declares a second backend build; a single-backend project never runs it
    reason: rule:static-route-discovery already makes this incomplete OpenAPI; the report states it as a limit on every other route check
pages:
  page-without-route:
    trigger: a .pw.html page inside a generate purpose that no registration renders
    severity: warning
    reason: it is generated, compiled, and unreachable, which reads as a missing registration rather than an intentional file
  route-without-page:
    trigger: a registration whose handler renders a page source that does not exist
    severity: error
templates:
  document-shell-count:
    trigger: zero or more than one requirement:nested-html-templates document shell across the generate.templates purposes
    severity: error
    status: api:cli-generate already fails on the second one; doctor states the resolved shell so the reader sees which file won
  error-page-missing:
    trigger: no 400, 404, or 500 page for flow:error-template-generation
    severity: note
    reason: api:problem-response has a framework fallback, so this is a finish-the-project item rather than a defect
  unreferenced-component:
    trigger: a component template no page or component includes, per data:ui-dependency-graph
    severity: note
rules:
  - every check here reads data:route-table and template sources, and none of them starts a server to find a route
  - a check is skipped, with a limit entry, when data:route-table is stale or unavailable, because a partial table produces false collisions
  - the report lists the effective routes even when no check fires, since the route list is the answer to "what does this application serve"
```
