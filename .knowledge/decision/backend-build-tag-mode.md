---
id: decision:backend-build-tag-mode
type: decision
title: The Backend Tag Selects Application Files, Not The Runtime Packages
---
The constraint that picks a transport is emitted onto application and generated artifacts; no framework package carries one, so both runtimes are compiled into every build and exactly one of them is wired to a listener.

```yaml
status: as-built, read against the tree 2026-08-10
serves: requirement:alternate-http-backend-readiness
verified_in_the_tree:
  no_backend_constraint: no //go:build line in pw, pwfast, pwruntime, or middlewares names a transport
  what_the_constraints_there_do_name: pwdev for the development runtime, tinygo and force_tinygo_logic for the compiler, pw_nogzip and pw_nozstd for the codecs; every one of them is an axis orthogonal to the transport
  sibling_shape: pwfast imports pwruntime and internal/requestorigin and does not import pw, and pw does not import pwfast, so neither drags the other in and both rest on the leaf of decision:shared-runtime-leaf
  where_the_tag_does_land: generated artifacts, per the tagging clause of decision:transport-source-transform, which emits the excluding constraint on net/http output and the including one on rewritten output
linked_is_not_serving:
  distinction: decision:transport-compatibility-fallback records that one binary serves one transport, and that is about which handler answers a port
  this_one: both runtime packages are present in that binary, because nothing excludes either
  why_it_is_not_waste: the pair is what the split below is made of, and the unused half of each is a handful of functions rather than a second framework
what_it_buys:
  the_handoff: pw owns startup — configuration binding, database startup, observability, and the validations that must fail before a port is bound — and publishes the transport-free result through pwruntime PublishChainSettings; pwfast Middlewares reads it through ResolvedChainSettings and assembles the request path only
  why_it_works_at_all: a package-level handoff between two runtimes requires both to be in one process, which is exactly what leaving the framework untagged provides
  no_second_binder: the second transport binds no configuration of its own, so a deployment reads its configuration once and neither runtime holds a copy that can drift from the other
  unblocked_here: requirement:pwfast-update-surface reached its first group this way rather than by moving the configuration binding into the leaf, which would have touched the generation pipeline for one entry
the_refusal_that_makes_it_safe:
  problem: a chain composed from an unpublished zero value has no recovery frame, no request ID and no security headers, and still serves requests and still looks like a chain
  answer: ResolvedChainSettings reports whether anything published, and pwfast Middlewares returns an error naming the cause rather than composing from zero
  consistent_with: policy:absent-rather-than-stubbed, applied to a value instead of to a surface
why_not_tag_the_framework:
  cost: a tagged pw would make the published settings unreachable from pwfast, so the two runtimes would each need their own configuration binding and their own startup, which is the duplication decision:shared-runtime-leaf exists to prevent
  and: a build tag excludes a whole file, so tagging the runtime packages would take their shared types, constants, and registrations with it, which is the same file-granularity fact decision:transport-source-transform records for application layout
relation_to_middleware_shape:
  decision:backend-specific-middleware: proposed, and describes framework middleware as one implementation per backend behind one build-tagged name
  as_built_instead: the shipped tree answers that with two untagged packages, middlewares for net/http and pwfast for the other, whose composition order is shared through pwruntime Compose rather than through a tagged name
  not_a_contradiction: that decision is about how a middleware body is written, and this one is about where the selecting constraint lands; the tagged-type arrangement it describes would still leave the packages themselves untagged
  open: whether that decision is updated to the shipped shape or the shipped shape moves toward it, which is a middleware question rather than a linkage one
consequences:
  - a cross-runtime handoff through a package-level value in the leaf is legal, and is the mechanism the chain settings already use
  - a build carries code for a transport it does not serve, which is a size cost and never a behavior one
  - the ordering rule of pwruntime Compose is shared rather than mirrored, so the two chains cannot run frames in different orders
  - an application file mixing a transport handler with other declarations breaks the artifact tagging rather than the framework's, which is why decision:transport-source-transform makes file granularity an application layout rule
```
