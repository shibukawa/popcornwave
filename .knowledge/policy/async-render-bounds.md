---
id: policy:async-render-bounds
type: policy
title: Async Render Bounds
---
Await boundary work stays bounded, cancellable, and server-confidential across both render branches of decision:automatic-async-render-selection.

```yaml
bound_delivery:
  fact: htmlbind.WithAsyncTimeout is read by the async coordinator, which the blocking path never builds, so the option alone bounds the streaming branch only
  streaming: pass the option, which bounds each boundary independently
  buffered: derive a render deadline from the same value and pass it as the htmlbind.WithContext context
  shape_difference: per boundary on one branch, per render on the other
  why_acceptable: nothing is committed on the buffered branch, so total wait is the number that matters, and api:async-html-value already started the work concurrently before the render began
  without_it: a chain forced onto the buffered branch waits on its boundaries until the request context ends, which is the stall the setting exists to prevent
rules:
  - bound one boundary with htmlbind.WithAsyncTimeout from data:html-render-config
  - bound a buffered render with a context deadline carrying the same value, per bound_delivery
  - use html.bot_async_timeout in place of html.async_timeout when api:client-classification reports a bot, since that request waits for every boundary before any byte leaves
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
