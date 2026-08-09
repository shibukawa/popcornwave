---
id: api:pwfast-package
type: api
title: pwfast Runtime Package
---
pwfast is the pw surface over a fasthttp request, the second half decision:transport-source-transform imports in place of pw when it rewrites a handler.

```yaml
package: github.com/shibukawa/popcornwave/pwfast
serves: requirement:pw-call-registration, whose second-package clause this is
status: first cut implemented 2026-08-09, against tinybind-go v0.4.10
shape:
  names: identical to the pw counterpart, so a rewritten call selector is unchanged and only the import line moves
  parameter: the request value first, named r, matching what the upstream emitter prints for this transport
  untagged: no build constraint, because only application files are tagged; a library behind one is invisible to an untagged go vet, go test, and gopls run
  transport: github.com/shibukawa/tinygodriver/fasthttp, the fork fasthttpbind itself uses, so the request type is one type rather than two that agree
implemented:
  bind: Parse
  api: WriteAPI and WriteAPIStatus
  problem: WriteProblem, whose document is byte-compatible with the net/http half
  html: WriteHTMLChain and WriteHTMLFragment
  stream: WriteStream and SetStreamErrorHandler
  shared_types: the composition types are htmlbind's and the error types are the module's shared leaf, aliased rather than redeclared, so an error crossing the seam still matches
buffered_render:
  fact: the chain renders into a buffer and commits after it succeeds, where the net/http half can stream
  why: committing first would trade a problem response for a half-written page, and the streaming path needs the flusher the deferred htmlupdate port holds
  cost: time to first byte, not bytes
absent_and_why:
  update_surface:
    what: WantsUpdate, WriteUpdate, WriteUpdateNavigate, Redraw, RedrawComponents, and live delivery
    reason: the upstream htmlupdate runtime reads the net/http request throughout and holds a flusher, and its port is deferred upstream
    not_stubbed: a stub that compiled and did nothing would hide the gap; an absent declaration is a build error naming it, which is what the refusal contract of decision:transport-compatibility-fallback does everywhere else
  document_shell:
    what: WriteHTML and WriteHTMLPage
    reason: both apply the registered document shell, and that registry is private to pw
    remedy: move it to a leaf both packages read, which is the move upstream already made for the error types
  redirect: api:redirect-response has no net/http half yet either, so there is nothing here to mirror
dependency_cost:
  added: the fasthttp fork brings a brotli encoder and a byte buffer pool into the module graph
  linking: unaffected for a project that imports neither package
verification: a test serves each entry through a real fasthttp server over an in-memory listener, rather than calling handlers with a synthesized request value, so what is asserted is what reaches the wire
```
