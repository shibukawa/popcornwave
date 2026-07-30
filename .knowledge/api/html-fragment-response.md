---
id: api:html-fragment-response
type: api
title: HTML Fragment Response API
---
pw.WriteHTMLFragment writes one generated template as a whole response with no shell, no merged head, and no chain, for requirement:html-fragment-rendering.

```yaml
surface:
  - WriteHTMLFragment(http.ResponseWriter, *http.Request, pw.HTMLFragment)
placement: pw, beside api:html-response WriteHTML
name_choice:
  picked: WriteHTMLFragment, keeping the Write prefix every surface that owns the response carries
  rejected: RenderFragment, which reads as a render helper and hides that this call commits the response
  effect: the name says nothing about skipping composition, so the Fragment suffix and this concept carry that distinction
behavior:
  - reject head contributions per decision:fragment-head-rejection before anything is rendered
  - render through htmlbind.Render into a buffer, so no wrapper participates and every await boundary settles in place
  - bound the render with a context deadline from html.async_timeout in data:html-render-config, per bound_delivery in policy:async-render-bounds
  - set Content-Type text/html; charset=utf-8
  - set Content-Length and apply configured compression exactly like the buffered branch of decision:automatic-async-render-selection
  - report boundary errors through api:logger with the same errors reporter as the page path
status:
  written: implicit 200 on the first write, like api:html-response
  other: not expressible here, because Content-Type must be set before a status is written; a non-200 partial goes out as api:problem-response
return: no normal error return, matching WriteHTML
errors:
  uncommitted: WriteProblem from api:problem-response, so the body is problem+json
  absent_leaf: htmlbind.ErrNoLeaf maps to 500
  unrecovered_boundary: htmlbind.UnrecoveredError maps to 500 and logs its cause
  never:
    - the registered HTML error page of api:problem-response
    - the document replacement of decision:unhandled-boundary-escalation
  why: an error document swapped into a region replaces that region with a whole page, and a non-2xx status is the signal swap libraries already act on by not swapping
not_done:
  - resolve the registered document of decision:implicit-document-shell
  - merge or emit head contributions
  - classify the client with api:client-classification, so nothing adds Vary User-Agent
  - write api:html-boundary-protocol framing or name requirement:external-boundary-runtime
unchanged_startup: a missing or duplicate default document is still the startup error decision:implicit-document-shell defines, so a fragment-only service still registers one
```
