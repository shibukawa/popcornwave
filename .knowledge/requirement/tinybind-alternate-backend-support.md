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
asked_upstream_2026_08_11:
  found_by: wiring the transform into api:cli-generate, which is the first consumer to run the derivation over a package that already holds the last run's generated code
  the_defect: the derivation is not idempotent over its own output, so a second generation of a package refuses on a file the first one wrote
  measured:
    run_one: a package with a body-reading request type and no generated binder emits its fasthttp binders and its derived handlers, both correct
    run_two: the same package with the first run's binder beside it refuses — "bindAsk is not transformable, captures r in a function literal" — and the run stops there
    why: GeneratePackage derives the handlers before emitting the binders, and AnalyzeTransform reads every file of the loaded package; a generated binder captures the request in a closure to read the body lazily, which the eligibility rule correctly calls an escape
    not_specific_to_this_framework: a run with Out unset writes its binders into the package it just analyzed, so the next run analyzes them
  the_ask: the derivation should skip generated files, which is the rule discovery already follows through GeneratedHeaders; an artifact-level entry for the fasthttp binders would also help, since GenerateArtifacts returns the derived handlers and no binders
  routes_around_it_that_were_tried_and_rejected:
    attempt_with_fallback: run the file-writing entry per package and take the binder file when it succeeds — rejected because it succeeds only on a package with no generated binder yet, so the file appears on the first generation and is swept on the second
    load_with_the_fasthttp_tag: would exclude the refusing generated binder and the authored handlers with it, leaving no usage sites for the plan; there is no way to set build flags on the load either
    staging_a_synthetic_package: deterministic, and it means generating from a package that is not the one that compiles, to work around one rule this framework already implements locally
  filtered_here_meanwhile: this framework runs AnalyzeTransform and RewriteTransform itself and drops every candidate from a generated file, so its own derivation is idempotent; only the binder phase, which it cannot reach, is not
  cost_until_it_lands: a derived handler calling pw.Parse compiles and answers 500 on the first request, because the fasthttp binder registry is populated from generated init functions; pw generate names the packages at the end of a run rather than leaving it to be found
```
