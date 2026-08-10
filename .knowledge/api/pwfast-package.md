---
id: api:pwfast-package
type: api
title: pwfast Runtime Package
---
pwfast is the pw surface over a fasthttp request, the second half decision:transport-source-transform imports in place of pw when it rewrites a handler.

```yaml
package: github.com/shibukawa/popcornwave/pwfast
serves: requirement:pw-call-registration, whose second-package clause this is
status: implemented 2026-08-10, against tinybind-go v0.5.1
shape:
  names: identical to the pw counterpart, so a rewritten call selector is unchanged and only the import line moves
  parameter: the request value first, named r, matching what the upstream emitter prints for this transport
  untagged: no build constraint, because only application files are tagged; a library behind one is invisible to an untagged go vet, go test, and gopls run
  transport: github.com/shibukawa/tinygodriver/fasthttp, the fork fasthttpbind itself uses, so the request type is one type rather than two that agree
implemented:
  bind: Parse
  api: WriteAPI and WriteAPIStatus
  problem: WriteProblem, whose document is byte-compatible with the net/http half
  html: WriteHTML, WriteHTMLPage, WriteHTMLChain, and WriteHTMLFragment
  problem_constructors: the pwruntime set, re-exported so a rewritten call finds the same names
  registration: RegisterHTMLDocument and RegisterHTMLErrorPage, reaching the one registry of decision:shared-runtime-leaf
  stream: WriteStream and SetStreamErrorHandler
  update: WantsUpdate, WriteUpdate, WriteUpdateNavigate, Redraw, RedrawComponents, Replace, and RegisterReloadable
  streaming: ServeUpdate for a streamed navigation over the same module entry the net/http half calls, and ServeLive over the shared protocol of decision:shared-runtime-leaf
  render_bounds: the async timeout and concurrency travel with the published settings, so a boundary is settled on the terms the deployment chose rather than on a default this half invented
  update_configuration:
    problem: every entry needs composed options built from this framework's own config type, which is bound by generated configbind code inside pw and cannot be named from here
    solved_by: the resolving runtime publishes what it resolved as a transport-free settings value in pwruntime, and this half reads it
    why_that_is_right: a settings file is not a transport concern, so the transport that read it and the transport that serves the request need not be the same one
    disabled: absent settings mean nothing enabled updates, and every entry treats that as not-an-update rather than as an error
  shared_types: the composition types are htmlbind's, aliased rather than redeclared
  problem_alias_was_wrong:
    found: 2026-08-09; a first draft aliased the module's two-field problem body under the name pw gives its own richer application-facing struct, so one name meant two types
    fixed: decision:shared-runtime-leaf shipped, and both halves now alias one declaration
buffered_render:
  fact: the chain renders into a buffer and commits after it succeeds, where the net/http half can stream
  why: committing first would trade a problem response for a half-written page, and the streaming path needs the flusher the deferred htmlupdate port holds
  cost: time to first byte, not bytes
live_converged_2026_08_10:
  first_cut_withdrawn: it called the module's live entry, which has none of the admission bound, watchdog, digest suppression, boundary bound, or close reasons the net/http half layers on, so it would have served a poorer stream with nothing to report the difference
  then_done_the_other_way: the protocol moved into the leaf, and both halves now read one set of decisions rather than two readings of the same document
  what_moved: the close reasons and media type, the lifetime jitter and watchdog, the per-client admission count, the keyed delivery digest and its per-process fallback, the manifest parse, and the four record writers
  what_each_runtime_keeps: setting response headers, obtaining a writer that flushes, naming the client for admission, and answering a pre-commit failure
  one_real_difference: the fasthttp body writer runs after the handler returned, so the loop reads nothing from the request value and is bounded by the watchdog rather than the request context; a client going away is noticed on the next write, which is the signal the other half falls back to once a record fails
absent_and_why:
  everything_absent_here: is absent rather than stubbed, per policy:absent-rather-than-stubbed
  redirect: api:redirect-response has no net/http half yet either, so there is nothing here to mirror
dependency_cost:
  added: the fasthttp fork brings a brotli encoder and a byte buffer pool into the module graph
  linking: unaffected for a project that imports neither package
verification: a test serves each entry through a real fasthttp server over an in-memory listener, rather than calling handlers with a synthesized request value, so what is asserted is what reaches the wire
```
