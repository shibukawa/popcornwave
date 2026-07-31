---
id: requirement:bot-synchronous-render
type: requirement
title: Bot Synchronous Render
---
A client that runs no boundary runtime receives the settled document instead of committed fallbacks, by taking the buffered branch that requirement:async-html-rendering already defines.

```yaml
problem:
  mechanism: the streaming branch commits the shell and every fallback first, and each boundary applies only when requirement:external-boundary-runtime executes in the client
  non_browser_effect: a client that runs no script keeps every fallback, so the fallback text is the final response body
  who_pays:
    search_indexer: indexes the loading state instead of the page content
    ai_crawler: ingests the loading state as the page
    preview_spider: falls back to loading text when it reads body content rather than head tags
    cli_and_library: receives template and tb-apply framing around placeholder text, which is neither the page nor an error
  not_a_browser_problem: a scriptless browser is rare and degraded on purpose; for these clients it is the only mode they have
solution:
  claim: no new render path is required
  reason: decision:automatic-async-render-selection already documents that the buffered branch renders every await boundary correctly, blocking until each one settles
  change: classify the client with decision:bot-client-classification and add its verdict as one more gate on the same branch selection
live_boundaries:
  behavior: the buffered branch renders a live boundary from its first delivery and stops watching, so a crawler receives one real render rather than a placeholder or an endless response
  quiet_source: a source that delivers nothing inside the bound leaves the fallback, which is the honest answer and not a failure
  consequence: requirement:live-html-rendering needs no bot-aware path either, for the same reason this requirement needs no async one
classification: api:client-classification over data:bot-user-agent-catalog
configuration: data:html-render-config
bounds: policy:async-render-bounds
non_goals:
  - different content, markup, or data for a classified bot; see the no-cloaking rule in decision:bot-client-classification
  - verifying that a declared crawler is the crawler it claims to be
  - classification driven by anything other than the User-Agent header
  - a bot-only render path, template variant, or prerender cache
criteria:
  - a request whose User-Agent matches data:bot-user-agent-catalog renders buffered even when the chain reports an await block
  - the buffered response for a bot carries Content-Length and no boundary framing, so no template or tb-apply element reaches it
  - a browser request to the same route with the same data still streams and still applies every boundary
  - the two branches produce the same text content once every boundary has settled
  - an await-capable response carries Vary User-Agent on both branches
  - html.bot_detection false makes a matching User-Agent stream like any browser
  - html.streaming false makes classification irrelevant, since every response is already buffered
  - a boundary that fails with no recover clause answers a bot with a real api:problem-response status rather than the 200 of decision:unhandled-boundary-escalation
  - a boundary that exceeds html.bot_async_timeout renders its recover clause, so a bot never receives an unsettled placeholder as content
  - classification reads the header only and starts no DNS lookup, no goroutine, and no external call
```
