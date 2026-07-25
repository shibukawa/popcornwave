---
id: api:html-response
type: api
title: HTML Response API
---
pw.WriteHTML completes the normal typed HTML response path so handlers do not branch on routine renderer errors.

```yaml
surface:
  - WriteHTML(http.ResponseWriter, *http.Request, generatedTemplate, generatedParams)
behavior:
  - select and set the HTML content type
  - apply configured compression
  - execute the generated typed template
  - record rendering failures in logs and traces
  - render api:problem-response HTML error pages when the response is not committed
commit_rule: after response commitment, record the error without attempting a replacement body
return: no normal error return
```
