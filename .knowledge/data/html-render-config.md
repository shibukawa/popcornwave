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
  html.bot_detection: bool gating requirement:bot-synchronous-render
  html.bot_async_timeout: duration bounding one await boundary on a classified bot request
  html.bot_user_agents: string list appended to data:bot-user-agent-catalog
defaults:
  html.streaming: "true"
  html.async_timeout: 3s
  html.async_concurrency: "0"
  html.bot_detection: "true"
  html.bot_async_timeout: 5s
  html.bot_user_agents: empty
sources: config.toml, environment variable, and CLI flag, like every other binding under decision:independent-runtime-config-bindings
read: api:html-response reads the binding from the request context per response
unparsed_default:
  problem: a registered binding reads back before anything parses it, and a zero value there is indistinguishable from an explicit setting
  effect_if_ignored: a zero streaming field would disable the feature in every test and every embedding that never parses configuration
  handling: registration seeds the target with the documented values, so the string defaults and the Go defaults come from one place
rules:
  - zero async_timeout means the request context is the only deadline
  - async_timeout bounds one boundary on the streaming branch and one whole render on the buffered branch, per the bound_delivery note in policy:async-render-bounds
  - zero or negative async_concurrency means unbounded, matching htmlbind
  - streaming false forces the buffered branch even when a chain can open a boundary, and htmlbind.RenderChain still produces a complete correct document
  - reject a negative duration at startup
  - bot_detection false skips classification entirely, so a crawler streams like a browser
  - streaming false makes bot_detection irrelevant, because every response is already buffered
  - zero bot_async_timeout falls back to async_timeout rather than meaning unbounded, so a misread key cannot hold a crawler connection open for the whole request deadline
  - bot_user_agents entries are lowercased and appended; a duplicate of a built-in token is accepted and has no effect
bot_timeout_tension:
  longer_side: an indexer waits far longer than a browser, and a timeout fallback baked into a buffered document is exactly the outcome requirement:bot-synchronous-render exists to prevent
  shorter_side: a preview spider abandons a slow response within a few seconds, and the buffered branch has no head start to offer it
  resolution: one key defaulting to 5s, above the browser bound and inside a typical spider budget
  rejected_for_now: per-category bounds keyed off the group in data:bot-user-agent-catalog
  revisit_when: measurement shows preview spiders abandoning responses that an indexer would have waited for
consumers:
  - api:html-response
  - api:client-classification
  - policy:async-render-bounds
```
