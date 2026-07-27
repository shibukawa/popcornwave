---
id: policy:async-render-bounds
type: policy
title: Async Render Bounds
---
Await boundary work stays bounded, cancellable, and server-confidential across both render branches of decision:automatic-async-render-selection.

```yaml
rules:
  - bound one boundary with htmlbind.WithAsyncTimeout from data:html-render-config
  - bound simultaneous boundary work with htmlbind.WithConcurrencyLimit from data:html-render-config
  - pass the request context so client disconnect and server shutdown stop the wait
  - an expired boundary renders recover with code timeout, or escalates through decision:unhandled-boundary-escalation
  - a failure with no recover clause never becomes a permanently stuck fallback
  - report every original error through htmlbind.WithErrorReporter into api:logger, including errors a recover clause handled
  - a recover subtree receives only pw.AsyncError; Go error text, SQL, and configuration never reach the page
  - an error gains a message or code only through an explicit PublicError projection
  - an external without a context parameter cannot be interrupted; the render abandons it and discards its result
  - work started by api:async-html-value stays the handler's to cancel
  - a boundary goroutine panic becomes that boundary's error and never reaches the recovery middleware
extends: policy:server-ui-security
```
