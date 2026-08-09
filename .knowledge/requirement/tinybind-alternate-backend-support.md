---
id: requirement:tinybind-alternate-backend-support
type: requirement
title: tinybind-go Support For A Second HTTP Backend
---
What was asked of system:tinybind for requirement:alternate-http-backend-readiness, and what v0.4.9 and v0.4.10 delivered against it.

```yaml
owner: system:tinybind
status: largely delivered; this is now a status record rather than an ask
surveyed: 2026-08-09 against v0.4.10, having first been written against v0.4.3
delivered:
  runtime: a fasthttp sibling package declaring the same names over the request value, with a shared transport-free leaf both surfaces alias
  shared_error_types: the problem, field-error, and HTTP-error types live in that leaf and are aliased rather than redeclared, so an error crossing between the surfaces still matches
  transform: eligibility, rewriting, refusal diagnostics with the chain from the handler, and a report-only run
  call_registry: transport slots on call patterns, plus a transport-only pattern for a call naming no model, which the module needed for its own error writer
  import_rewrites: a map from an authored import path to the one the generated file imports under the original local name
  router: a vendored fasthttprouter beside the fasthttp fork, with a configurable target naming import, qualifier, type, registration function, and catch-all spelling
  build_tags: generation emits them, so the net/http artifacts and the rewritten ones never compile together
  stream: the callback entry point, on both transports
  docs: an application-author guide and a framework-owner guide, the latter written for exactly this framework's position
already_neutral_and_unchanged:
  packages:
    - htmlbind and its delta subpackage, whose render entry points all take io.Writer
    - jsonbind, an append-to-bytes API
    - sqlbind, configbind, dynamobind, firestorebind, minitoml, cliparser
  significance: the heaviest dependency this framework has on the module needed no port, which is why the delivered scope is smaller than the dependency list suggested
resolved_differently_than_asked:
  handler_arity:
    asked: this requirement called it the thing to settle first, since every emitter and analyzer assumed a writer and a request as two values
    answer: the transform rewrites both identifiers to the same context, so arity collapses as a consequence of substitution rather than as a modelled change; only the printed signature moves, which routetree already parameterized
  no_adapter:
    asked_here: a compatibility fallback, per the proposal decision:transport-compatibility-fallback records
    delivered: a refusal contract instead, which then forced the transform to close over the same-package call graph
what_is_left:
  here_not_upstream: requirement:pw-call-registration and the pw fasthttp package, which the module cannot supply and explicitly hands to a framework owner
  tinygo: whether the fork and the vendored router build under the target this framework pins, which decision:tinygo-042-baseline makes a real question
  matching_semantics: a route table meaning the same thing on both routers, which no transform can rewrite and which rule:route-and-template-checks is the place for
```
