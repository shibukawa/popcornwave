---
id: api:html-response
type: api
title: HTML Response API
---
pw.WriteHTML completes the normal typed HTML response path so handlers do not branch on routine renderer errors.

```yaml
surface:
  - WriteHTML(http.ResponseWriter, *http.Request, pw.HTMLFragment)
behavior:
  - select and set the HTML content type
  - resolve decision:implicit-document-shell and execute it with the page through api:render-html-chain
  - pick the buffered or streaming branch through decision:automatic-async-render-selection
  - apply configured compression on the buffered branch only, per decision:streaming-response-compression
  - apply data:html-render-config bounds through policy:async-render-bounds
  - record rendering failures in logs and traces
  - render api:problem-response HTML error pages when the response is not committed
commit_rule: after response commitment, record the error without attempting a replacement body
async: an await-capable chain streams without changing this surface, per requirement:async-html-rendering
return: no normal error return
boundary: handwritten handlers see neither pw.HTMLWrapper nor templates/document.pw.html
```
