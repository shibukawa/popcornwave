---
id: data:html-render-config
type: data
title: HTML Render Runtime Config
---
The `html` binding bounds and gates progressive rendering for requirement:async-html-rendering.

```yaml
prefix: html
registration: pw.RegisterConfig[HTMLConfig]("html") beside the other built-in bindings
keys:
  html.streaming: bool
  html.async_timeout: duration bounding one await boundary
  html.async_concurrency: maximum simultaneous boundary work per render
defaults:
  html.streaming: "true"
  html.async_timeout: 3s
  html.async_concurrency: "0"
sources: config.toml, environment variable, and CLI flag, like every other binding under decision:independent-runtime-config-bindings
read: api:html-response reads the binding from the request context per response
unparsed_default:
  problem: a registered binding reads back before anything parses it, and a zero value there is indistinguishable from an explicit setting
  effect_if_ignored: a zero streaming field would disable the feature in every test and every embedding that never parses configuration
  handling: registration seeds the target with the documented values, so the string defaults and the Go defaults come from one place
rules:
  - zero async_timeout means the request context is the only deadline
  - zero or negative async_concurrency means unbounded, matching htmlbind
  - streaming false forces the buffered branch even when a chain can open a boundary, and htmlbind.RenderChain still produces a complete correct document
  - reject a negative duration at startup
consumers:
  - api:html-response
  - policy:async-render-bounds
```
