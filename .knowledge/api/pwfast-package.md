---
id: api:pwfast-package
type: api
title: pwfast Runtime Package
---
pwfast is the pw surface over a fasthttp request, the second half decision:transport-source-transform imports in place of pw when it rewrites a handler.

```yaml
package: github.com/shibukawa/popcornwave/pwfast
serves: requirement:pw-call-registration, whose second-package clause this is
status: implemented 2026-08-09, against tinybind-go v0.5.0
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
  shared_types: the composition types are htmlbind's, aliased rather than redeclared
  problem_alias_was_wrong:
    found: 2026-08-09; a first draft aliased the module's two-field problem body under the name pw gives its own richer application-facing struct, so one name meant two types
    fixed: decision:shared-runtime-leaf shipped, and both halves now alias one declaration
buffered_render:
  fact: the chain renders into a buffer and commits after it succeeds, where the net/http half can stream
  why: committing first would trade a problem response for a half-written page, and the streaming path needs the flusher the deferred htmlupdate port holds
  cost: time to first byte, not bytes
absent_and_why:
  update_surface:
    what: WantsUpdate, WriteUpdate, WriteUpdateNavigate, Redraw, RedrawComponents, and live delivery
    plan: requirement:pwfast-update-surface, which splits it into what needs nothing, what needs a request reader, and what is genuinely blocked; the first two are larger than the third
    reason_first_given: the runtime holds a flusher and reads the net/http request throughout, which is true of the streaming half and no longer of the rest
  everything_absent_here: is absent rather than stubbed, per policy:absent-rather-than-stubbed
  redirect: api:redirect-response has no net/http half yet either, so there is nothing here to mirror
dependency_cost:
  added: the fasthttp fork brings a brotli encoder and a byte buffer pool into the module graph
  linking: unaffected for a project that imports neither package
verification: a test serves each entry through a real fasthttp server over an in-memory listener, rather than calling handlers with a synthesized request value, so what is asserted is what reaches the wire
```
