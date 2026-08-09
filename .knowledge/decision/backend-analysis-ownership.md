---
id: decision:backend-analysis-ownership
type: decision
title: The Analysis Engine Is Upstream, The Vocabulary Is Here
---
system:tinybind owns one typed analysis of handler bodies and the emitter it feeds; Popcorn Wave owns the vocabulary that analysis is run against and the diagnostics it produces, connected by the adapter mechanism requirement:httpbinder-extensible-route-analysis already defines.

```yaml
status: proposed
serves: decision:transport-source-transform and rule:transport-handle-checks
the_question: whether the static checks and the transform's analysis are built upstream, or here beside the checks that already exist
why_not_two_engines:
  fact: rule:transport-handle-checks and decision:transport-source-transform need the same facts, being where w and r flow, which accessors are read, and which flow leaves the framework surface
  failure_mode: built separately they drift, and the drift is the worst possible one, a check that passes a handler the transform then cannot take
  precedent: decision:shared-check-catalog was written against exactly this, one condition implemented twice diverging in wording, severity, and edges
  cost: a second typed pass re-type-checks the package system:tinybind already type-checked during generation, which is the expensive part of both
why_not_all_upstream:
  vocabulary: the surfaces the rule permits are pw's, and pw is the layer above; encoding them upstream inverts the dependency decision:root-pw-api sets
  identifiers: the PW numbering, severities, remedies, and generated documentation belong to data:diagnostic-check, which is this framework's
  audience: another consumer of the module needs the engine and none of the above
  iteration: a check is a product decision here, and needing an upstream release per check is friction that ends with checks not being written
split:
  upstream:
    - the typed analysis of handler bodies, parameterized by a declared vocabulary rather than by hard-coded identifiers
    - an exported result, which requirement:httpbinder-extensible-route-analysis already lists as a candidate followup
    - the net/http accessor mapping, which is backend-generic knowledge and not this framework's
    - the emitter, already symbol-parameterized in routetree and needing the same in generator
  here:
    - the pw vocabulary, supplied as adapter data the way api:cli-generate already supplies the router adapter
    - the rule:transport-handle-checks catalog and the surfacing of the upstream report through this framework's own tooling
    - orchestration: which directories, which artifacts merge, and the build constraint of decision:transport-source-transform, all of which internal generation already owns
  shape: adapters are data and the engine executes none of it, which is the security rule requirement:httpbinder-extensible-route-analysis already states
settled_2026_08_09:
  outcome: tinybind-go v0.4.9 shipped the analysis, the transform, and a report-only run, parameterized by a call registry the caller supplies
  verdict: the split this decision proposed is what was built, so the sequencing below is history rather than a plan
  what_remains_here: requirement:pw-call-registration, which is the vocabulary half of exactly this split
sequencing_as_proposed:
  prototype_here_first:
    why: the exported result cannot be designed from speculation; writing the checks against the examples of this repository is what says what it must carry
    then: move the engine upstream once its shape is known, and keep the vocabulary and the catalog here
    trap: a prototype that quietly becomes permanent is the two-engines outcome this decision rejects, so it is built to be upstreamed or discarded rather than kept
  cheap_first_step: run the checks over this repository's own examples, scaffolds, and tutorial, which requirement:alternate-http-backend-readiness already names as the thing to do before anything else
consequences:
  - the transform and the checks read one analysis, so a handler the check clears is one the transform can take, by construction rather than by agreement
  - system:tinybind gains a capability its other consumers can use, rather than one shaped around this framework
  - requirement:tinybind-alternate-backend-support gains this as its analysis item, beside the runtime and emitter ones
```
