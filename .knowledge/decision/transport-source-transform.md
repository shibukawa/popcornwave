---
id: decision:transport-source-transform
type: decision
title: Adopting The Upstream Source Transform
---
The net/http source stays the authored form and the fasthttp build is generated from it; system:tinybind owns the rewriter, and what remains here is the vocabulary it rewrites against and the layout its file granularity forces.

```yaml
status: upstream shipped in tinybind-go v0.4.9, documented in v0.4.10; adoption here proposed
upstream_owns:
  eligibility: a whitelist over every occurrence of the transport identifiers, decidable and conservative, refusing rather than analyzing what it does not recognize
  transitivity: admission closes over the same-package call graph, so a shared error or render helper is carried rather than refusing its callers
  rewriting: the two transport parameters collapse to one context, and a recognized call drops the arguments carrying no semantic value
  selectors: a named, enumerated rewrite table, never a general rule about methods on the request
  refusal: a generation error naming the occurrence, its chain from the handler, and a remedy
  report_only: a run that writes nothing and lists what a fasthttp build would refuse
what_this_framework_still_owns:
  vocabulary: requirement:pw-call-registration, without which every pw call is an unrecognized one
  second_runtime: the pw surface over the fasthttp request value, reached by an import rewrite
  layout: the file granularity below, which is a project convention this framework's scaffolds should already satisfy
  surfacing: rule:transport-handle-checks, which decides how the upstream report reaches a developer here
file_granularity:
  fact: a build tag excludes a whole file, so an authored file holding a transport handler beside a type, const, or var declaration cannot be tagged without taking those with it, and both builds need them
  consequence: a transport handler belongs in a file containing nothing else, which is a real constraint on application layout rather than a style preference
  enforced_upstream: the rewriter reports every authored file mixing the two and names the declarations that would be lost
  owed_here: api:cli-init scaffolds, the examples, and requirement:tutorial-continuity must produce that layout, or this framework teaches the shape its own transform rejects
  framework_interior: the upstream framework-tag-boundary guidance answers the same constraint differently for pw itself, keeping one import path and tagging the type rather than its users, per decision:backend-specific-middleware
tagging_is_upstream_now:
  fact: with a backend selected, generation emits the constraint itself; net/http artifacts carry the excluding tag and the rewritten ones carry the including tag, and a run with no backend selected emits none
  local_consequence: the build-tag application added to this framework's generation predates that capability, so the two would both run once the transform options are wired
  collision_closed_2026_08_09:
    what: the generator writes its constraint one line below the generated-code header, where a check for a leading one misses it and adds a second; a file carrying two of them does not compile
    fixed_by: reading the whole header rather than the first line, and deferring to any constraint already present rather than reconciling with it
    second_defect_found: merging artifacts rebuilds a file from its declarations and dropped the header with it, so a constraint the generator emitted was silently lost; the merge now carries it and refuses artifacts that disagree
    settled_2026_08_11:
      was_the_plan: delete this framework's application of the tag once the transform options were wired, on the reading that the generator would then own it entirely
      what_wiring_it_showed: the generator emits the constraint from the entry point that writes files, and this framework generates through the one that returns artifacts, which emits it on neither half
      so: the derived handlers arrive already carrying the including tag and are left alone, and the excluding tag on the net/http half is still this framework's to apply, per file and by its imports
      deferring_was_the_right_shape_anyway: it is what lets the two coexist rather than needing to be ordered
generated_body_is_a_copy:
  fact: the emitted handler carries a rewritten copy of the authored statements
  cost: a panic or a profile points at generated source rather than at the authored file
  bearing_here: policy:generated-artifacts already makes generated Go reproducible output, and this is the first case where it carries application logic rather than framework glue
given_up_upstream:
  given_up:
    - incremental migration; adoption is all-or-nothing per build and a service cannot move route by route while running
    - a mixed process; one binary serves one transport
    - net/http-shaped third-party middleware in a fasthttp build, since nothing wraps it
  bearing_here: the last one is the sharpest, because policy:web-middleware invites exactly that composition
```
