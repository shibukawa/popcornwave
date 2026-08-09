---
id: decision:dev-browser-runtime-scope
type: decision
title: Development Browser Runtime Scope
---
Development-only browser JavaScript is a permitted third class, separate from the framework runtime of requirement:framework-script-assets and from the application-owned scripts of concept:interaction-cost-ladder, and it is bounded by never reaching an api:cli-build artifact.

```yaml
status: accepted
supersedes: the reading that pw ships no browser JavaScript, which requirement:external-boundary-runtime already ended
context:
  - requirement:external-boundary-runtime ships a runtime to every page, because requirement:async-html-rendering has to apply completions in the browser
  - the position that remains true is narrower: no hydration, no client router, no client state store, and nothing an application must adopt
  - requirement:dev-console and requirement:dev-error-overlay need browser code of a kind neither existing class describes
classes:
  framework:
    concept: requirement:framework-script-assets
    served_by: the application, from the reserved revision-stamped path
    constraints: fixed per framework version, immutably cacheable, no request data
  application:
    concept: concept:interaction-cost-ladder authored_islands
    served_by: the application, from its public tree
    constraints: the ladder's, so a tier is justified by an interaction the tier below cannot express
  development:
    served_by: requirement:dev-console, except for the one module described below
    constraints: this decision
development_class_rules:
  - never present in an api:cli-build artifact, which the pwdev build constraint of policy:dev-console-boundary enforces
  - never required for the application to work; every page renders the same without it
  - not bound by the ladder, because the ladder governs what an application spends on its users and the console has one user who is the developer
  - free to use an ordinary bundler and third-party components, since it is never delivered to anyone but the developer
in_application_exception:
  what: one dev module the framework serves under pwdev, which requirement:dev-error-overlay and requirement:dev-console-launcher need because both must run inside the application's own pages
  still_one: a second behavior did not earn a second module; they share a console address and a stream, and splitting them would open two
  and_one_asset: the module references data:dev-launcher-mark, so the pwdev set is no longer only JavaScript and the asset handler picks a content type by extension
  how: the requirement:framework-script-assets core dynamically imports it, exactly as it imports any other capability module
  why_not_head_injection: decision:implicit-document-shell states that no framework code injects into the head, and this keeps that true
  why_not_a_scaffold_tag: a dev-only tag in a committed templates/document.pw.html would ship to production in every project that forgot to remove it
  revision: the digest is computed from the bytes actually served, so the pwdev script set gets its own URL with no constant to bump
  limit: a project whose document shell dropped the runtime reference gets no overlay, and the console index still reports the state
open:
  - whether the console UI is one bundle or one per pane, which is a packaging question rather than a boundary one
```
