---
id: decision:unhandled-boundary-escalation
type: decision
title: Unhandled Boundary Escalation
---
An await boundary that fails with no recover clause replaces the document body with an api:error-renderer page, instead of leaving its fallback on screen forever.

```yaml
status: accepted
problem:
  current: htmlbind drops such a boundary, so the committed fallback becomes the final content
  effect: a page shows a loading state permanently, with nothing on screen saying why
  intent_mismatch: an author who omitted recover did not choose an endless fallback; they said nothing about failure
  worse_when_blocking: the synchronous entry produces a finished document containing that same unresolvable loading state
model:
  omitted_recover: delegates the failure, and the framework answers with an error page
  present_recover: contains the failure inside its own boundary and leaves the rest of the page alone
  cancellation: neither, because nobody is reading the response
target:
  where: the children of the document body
  no_identifier_needed: an error page replaces everything below the shell by definition, so nothing has to name a narrower region
  rejected: wrapping the chain slot in an identified element, which would change the DOM shape of every page for one failure path
branches:
  streaming:
    constraint: the status went out with the shell, so the response stays 200 and only the body changes
    detection: an UnrecoveredError is reported only after the initial pass, so the document is committed by construction rather than by checking
    consequence: a monitor, a crawler, or a cache sees success; failure visibility belongs to api:logger, not to the status line
    envelope: api:html-boundary-protocol document envelope
  buffered:
    constraint: nothing is committed, so this failure can still carry a real status
    behavior: answer as an ordinary api:problem-response error page rather than swapping anything
  asymmetry_is_deliberate: one branch can still tell the truth in its status line, and it should
error_page:
  registration: pw.RegisterHTMLErrorPage installs a resolver receiving the mapped problem, never the original error
  builtin: a minimal status-and-title page when no resolver is registered, so the escalation never depends on application setup
  recursion: a resolver whose own render fails falls back to the builtin rather than escalating again
  chain: the buffered branch renders the error page through the same wrapper chain, so it keeps the document shell
framing:
  markup: "<template data-tb-document>page</template><tb-apply-document></tb-apply-document>"
  discipline: the same trailing marker rule as a completion, for the same parser reason
interaction:
  pending_siblings: their placeholders leave with the body, and their completions then find no target and no-op
  ordering: a sibling that already applied is discarded too, because the page is being replaced rather than patched
  stream_end: streaming stops after the escalation; nothing more may be written
  no_javascript: a client without the runtime keeps the stuck fallback, since the swap needs the runtime that api:html-boundary-protocol describes
risk:
  escalation: a non-critical widget can take down a whole page that was otherwise fine
  mitigation: recover is the documented way to contain a boundary, and generation could later warn on a boundary with neither recover nor an explicit opt-out
```
