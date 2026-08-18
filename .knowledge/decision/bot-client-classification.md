---
id: decision:bot-client-classification
type: decision
title: Bot Client Classification
---
Popcorn Web classifies a request as browser or bot from the User-Agent header alone, and uses the verdict only to choose a render branch.

```yaml
status: accepted
scope: requirement:bot-synchronous-render, and nothing else; classification never reaches routing, authorization, rate limiting, or content
signal:
  chosen: the User-Agent request header
  why: it is the only bot evidence present on every request, free to read, and stable across transports
rejected_signals:
  ip_range_or_reverse_dns:
    why_not: a per-request DNS round trip inside the render path, and it identifies only the large verified crawlers
    gap: says nothing about curl, a scraper library, or an OGP spider, which are most of the population this requirement serves
  accept_header:
    why_not: bots send */* and so does fetch, while browsers and bots both send text/html on a navigation
  javascript_challenge:
    why_not: the defining property of the clients being served is that they run no script, so a script cannot ask them anything
    does_not_cover: a noscript challenge, which activates precisely when scripting is off and is therefore a different question; decision:scriptless-browser-detection settles that one separately
  explicit_request_header_or_query_override:
    why_not: an unauthenticated switch on render behavior is a trust hole with no operator benefit; data:html-render-config already covers every real need
test:
  order: evaluated as written, first match wins
  steps:
    - id: disabled
      when: html.bot_detection is false
      verdict: browser
    - id: absent
      when: the header is missing or empty after trimming
      verdict: bot
      why: a browser always sends one; an omitted header is a script that never set it
    - id: token
      when: the lowercased header contains any token in data:bot-user-agent-catalog
      verdict: bot
      why: catches the bots that present a full browser-shaped User-Agent
    - id: shape
      when: the header does not begin with "mozilla/"
      verdict: bot
      why: every mainstream browser prefixes Mozilla/5.0 for historical reasons, and effectively no CLI or client library does
    - id: default
      verdict: browser
shape_rule_value:
  covers_without_a_list_entry: curl, Wget, HTTPie, python-requests, urllib, httpx, aiohttp, Scrapy, Go-http-client, okhttp, Java, Apache-HttpClient, axios, node-fetch, got, GuzzleHttp, libwww-perl, Faraday, RestSharp, PowerShell, PostmanRuntime
  effect: an unknown tool released tomorrow is classified correctly with no catalog change, so data:bot-user-agent-catalog only has to carry the browser-shaped bots
substring_discipline:
  rejected: matching the bare substring "bot"
  reason: real device names contain it, so an Android phone reporting CUBOT would be classified as a crawler
  rule: every token is specific enough that no shipping browser User-Agent contains it, which is a property data:bot-user-agent-catalog must preserve on every addition
no_verification:
  fact: a User-Agent is self-declared and nothing here checks it
  why_acceptable: the only outcome is which render branch runs, and both branches render the same templates with the same data, so a forged header buys a slower first byte and nothing else
  corollary: this must never become an input to an access decision, because it would then be an unauthenticated one
no_cloaking:
  rule: both branches render one chain with one set of data, so the settled text content is identical
  why: differing content by User-Agent is cloaking, which search engines penalize; differing delivery for the same content is the documented dynamic-rendering pattern
  enforcement: classification is read at branch selection only, and is not available to a template or to a generated parameter
error_asymmetry:
  streaming: a boundary failing with no recover clause is reported after commit, so decision:unhandled-boundary-escalation swaps the body under a 200
  buffered: nothing is committed, so the same failure answers as a real api:problem-response status
  consequence: a classified bot receives the truthful status, which is what a crawler or a monitor should act on; this is a benefit of the branch, not a separate feature
misclassification_cost:
  browser_as_bot: a correct page with a later first byte and no progressive delivery
  bot_as_browser: fallback text becomes the indexed or piped content
  asymmetry: the first is a performance regression and the second is a correctness one, so ties are resolved toward bot
known_limit:
  what: a scraper that copies a current browser User-Agent is indistinguishable and takes the streaming branch
  why_tolerated: it is the ceiling of every header-based method, and a client presenting itself as a browser is asking to be treated as one
  no_escalation: closing this needs behavioral fingerprinting, which is out of scope and would put a heuristic on the render hot path
cache_correctness:
  problem: one URL now has two byte representations, so a shared cache could serve a streamed body to a crawler or a buffered body it never varies again
  rule: set Vary User-Agent on any response whose chain reports an await block, on both branches
  scoped_deliberately: a chain with no await block has one representation, so it keeps a cacheable response with no Vary
  cost: Vary User-Agent collapses shared-cache hit rate for the varying pages, which is the price of two representations and is why it is not applied globally
  interaction: decision:streaming-response-compression already adds Vary Accept-Encoding; both values apply
headless_browsers:
  decision: not classified as bots by default
  why: HeadlessChrome and a Playwright or Puppeteer client execute the runtime, so streaming works for them
  risk: a screenshot or PDF tool may capture before the boundaries settle
  handling: add the token through html.bot_user_agents when that tool matters, rather than making every automated browser pay for it
robots_txt_only_tokens:
  fact: Google-Extended and Applebot-Extended are robots.txt control tokens and never appear as a request User-Agent
  rule: keep them out of data:bot-user-agent-catalog, since an entry that cannot match is a maintenance cost with no effect
```
