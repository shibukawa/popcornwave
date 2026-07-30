---
id: api:client-classification
type: api
title: Client Classification API
---
pw.IsBot answers whether a request came from a non-interactive client, using the one test decision:bot-client-classification defines.

```yaml
surface:
  - IsBot(*http.Request) bool
placement: pw, beside the other request accessors of api:request-context-accessors
no_middleware:
  choice: a pure function over the request, evaluated where the answer is needed
  why: nothing has to run before it, so there is no ordering requirement, no capsule field, and no way for a route to miss the classification
  cost: repeated calls repeat the scan, which is bounded and allocation-free on the browser path
behavior:
  - read the User-Agent header and apply the decision:bot-client-classification test
  - resolve html.bot_detection and html.bot_user_agents from data:html-render-config through the request context
  - return false when no configuration binding is present, so an embedding that never parses configuration keeps browser behavior
callers:
  framework: api:html-response, as the third gate of decision:automatic-async-render-selection
  application:
    intent: choosing a cheaper query shape, suppressing an analytics beacon, or logging crawler traffic
    limit: never an authorization or rate-limit input, per the no-verification rule in decision:bot-client-classification
    limit_rendering: a handler must not branch on it to change what a page says, per the no-cloaking rule
not_exposed:
  to_templates: no generated parameter, external function, or template accessor carries the verdict
  why: making it reachable from a template turns a delivery decision into a content decision, which is exactly the cloaking this design forbids
testing:
  browser: any User-Agent beginning with Mozilla/ and matching no token
  bot: an empty header, a token from data:bot-user-agent-catalog, or any non-Mozilla string
  determinism: no clock, network, or shared state, so a classification test needs no fixture beyond the header
```
