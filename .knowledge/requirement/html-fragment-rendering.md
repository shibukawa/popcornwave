---
id: requirement:html-fragment-rendering
type: requirement
title: HTML Fragment Rendering
---
A handler answers a partial-HTML request with one template's markup only, so an htmx-style swap library can insert it into a document that already exists.

```yaml
motivation: an interaction replacing one region needs that region, and wrapping it in decision:implicit-document-shell would swap a second html, head, and body into the live page
surface: api:html-fragment-response
handler_delta:
  required:
    - call the fragment surface instead of api:html-response WriteHTML
  unchanged:
    - handler registration in flow:handler-registration
    - request binding, authentication, and middleware order
    - generated Fragment and Params construction
    - error return style
output:
  is: what the named template writes, with nothing before or after it
  is_not:
    - the document shell of decision:implicit-document-shell
    - a merged head from api:render-html-chain
    - a wrapper or layout chain
    - api:html-boundary-protocol framing or a requirement:external-boundary-runtime reference
head_contributions: decision:fragment-head-rejection
delivery: decision:buffered-fragment-delivery
client_contract:
  owner: the application, since the route, the trigger attributes, and the swap target all live in application templates
  framework_gives: markup and status
  distinction: flow:partial-refresh is the framework-managed patch endpoint with its own envelope, versions, and data:ui-dependency-graph; this surface carries no envelope, no boundary id, and no version
  consequence: the framework cannot tell a stale swap from a current one here, so ordering and cancellation belong to the swap library
example:
  project: examples/htmx_fragment
  library: htmx loaded from a pinned CDN URL, named only by the document shell, so no route knows which swap library called it
  reuse: the page composes the same TaskPanel and TaskList components the partial routes render alone
  routes:
    filter: GET /tasks answers the list region for the filter box
    create: POST /tasks answers the panel region, with field errors rendered into the form at 200 because a swap library ignores a non-2xx
    remove: DELETE /tasks/{id} answers the list region, and a row already gone stays a 404 rather than a pretended success
    summary: GET /tasks/summary settles an await boundary in place, so the declared fallback never reaches the browser and htmx owns the waiting state
    head_rejection: GET /tasks/broken renders a component with a scoped style block and is refused per decision:fragment-head-rejection
non_goals:
  - a wrapper chain on this path; a caller wanting nesting composes it inside the template
  - a status other than 200; a failing partial goes out as api:problem-response
  - progressive delivery of a fragment, which decision:buffered-fragment-delivery rules out
  - reusing the fragment leaf as a page, which still needs the shell path
acceptance:
  - the response body equals the template output byte for byte
  - a template declaring head contributions fails as a 500 before any byte
  - a template with an await boundary emits settled markup carrying no placeholder and needing no client runtime
  - an unrecovered boundary failure becomes a problem response, since nothing is committed
  - no fragment response varies on User-Agent and no request is classified by api:client-classification
  - Content-Length and compression behave like the buffered branch of decision:automatic-async-render-selection
  - the page path of api:html-response keeps its current bytes, so adopting this surface changes nothing about existing routes
```
