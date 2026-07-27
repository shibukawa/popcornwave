---
id: data:bot-user-agent-catalog
type: data
title: Bot User Agent Catalog
---
The framework-owned token list decision:bot-client-classification matches against, covering the bots that present a browser-shaped User-Agent.

```yaml
matching:
  normalize: lowercase the header once per classification
  compare: substring containment against lowercase tokens
  structure: tokens compiled once when data:html-render-config parses, never rebuilt per request
  cost: one lowercase pass plus a bounded scan, no allocation on the browser path
membership_rule:
  requirement: no shipping browser User-Agent may contain the token
  procedure: reject a token that is a bare word likely to appear in a device or product name, and prefer the vendor-qualified form
  scope: browser-shaped bots only, because the shape rule in decision:bot-client-classification already covers every tool that omits the Mozilla prefix
tokens:
  search:
    - googlebot
    - google-inspectiontool
    - storebot-google
    - adsbot-google
    - mediapartners-google
    - bingbot
    - bingpreview
    - msnbot
    - slurp
    - duckduckbot
    - duckassistbot
    - baiduspider
    - yandexbot
    - yandexrenderresourcesbot
    - sogou
    - seznambot
    - qwantbot
    - petalbot
    - applebot
    - naver
    - yeti/
  ai:
    - gptbot
    - oai-searchbot
    - chatgpt-user
    - claudebot
    - claude-web
    - claude-user
    - claude-searchbot
    - anthropic-ai
    - perplexitybot
    - perplexity-user
    - ccbot
    - bytespider
    - amazonbot
    - meta-externalagent
    - cohere-ai
    - diffbot
    - youbot
    - timpibot
    - omgili
  social_preview:
    - facebookexternalhit
    - facebookcatalog
    - meta-externalfetcher
    - twitterbot
    - slackbot
    - slack-imgproxy
    - discordbot
    - telegrambot
    - whatsapp
    - linkedinbot
    - pinterest
    - redditbot
    - skypeuripreview
    - embedly
    - iframely
    - vkshare
    - mastodon
    - bitlybot
    - nuzzel
    - quora link preview
  windows_http_stacks:
    note: the notable client libraries that do claim Mozilla, inherited from the Internet Explorer engine they were built beside, so the shape rule cannot reach them
    tokens:
      - winhttp
      - wininet
      - ms-office
      - microsoft office
      - microsoft outlook
  seo_and_monitoring:
    - ahrefsbot
    - semrushbot
    - mj12bot
    - dotbot
    - dataforseobot
    - screaming frog
    - w3c_validator
    - pingdom
    - uptimerobot
    - statuscake
  generic_shape:
    note: a browser-shaped agent that still names itself, kept last because these are the widest tokens in the list
    tokens:
      - "+http"
      - crawler
      - spider
      - feedfetcher
      - archive.org_bot
    caution: "+http" matches the contact URL convention that a well-behaved crawler follows, and no browser emits it
excluded:
  robots_txt_only:
    tokens: [google-extended, applebot-extended]
    why: robots.txt control tokens that never appear as a request header, per decision:bot-client-classification
  bare_bot:
    token: bot
    why: device names such as CUBOT contain it, so it would misclassify real phones
  headless_browsers:
    tokens: [headlesschrome, chrome-lighthouse, puppeteer, playwright]
    why: they execute requirement:external-boundary-runtime, so streaming already serves them
    opt_in: add through html.bot_user_agents when a capture tool reads the page before the boundaries settle
extension:
  key: html.bot_user_agents in data:html-render-config
  semantics: appended to this list, never replacing it
  removal: not supported, because a built-in token colliding with a real browser is a defect in this catalog rather than something an application should route around
maintenance:
  ownership: framework, versioned with the module, so an application gains new crawlers by upgrading
  drift: a bot missing from this list still renders correctly, only progressively, so a stale list degrades rather than breaks
  addition_test: every token is asserted against a fixture of current browser User-Agents, so a colliding addition fails at build time
consumers:
  - api:client-classification
  - decision:bot-client-classification
```
