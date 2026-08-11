---
id: requirement:browser-report-ingest
type: requirement
title: Browser Report Ingest
---
A deployment names a framework-owned endpoint in `Reporting-Endpoints`, and every report a browser delivers there becomes one api:logger record, so the enforcing policy this framework now ships by default stops breaking pages invisibly.

```yaml
status: requirements recorded 2026-08-10, unimplemented
priority: should
source: user request 2026-08-10, collect client errors through the Reporting-Endpoints header; logging what arrives is enough
driving_case:
  what: pwruntime DefaultContentSecurityPolicy ships an enforcing policy to every project that names none, on the stated reasoning that a CSP shipped empty is one nobody had
  consequence: a page loading third-party script is broken by a default nobody chose, and nothing on the server says so
  documented_dead_end: the website security-headers guide shows report-uri /csp-report in its report-only example, and no build of this framework serves that path, so the one documented way to learn what a policy would break points at a 404
  therefore: the endpoint is what makes both the shipped default and the content_security_policy_report_only key usable rather than decorative
what_the_mechanism_actually_carries:
  verified: 2026-08-10 against the W3C Reporting API specification and caniuse
  types: csp-violation, deprecation, intervention, crash, coep, coop, permissions-policy-violation, integrity-violation
  not_carried:
    javascript_errors: window.onerror and unhandledrejection are not report types; no Reporting API configuration delivers an uncaught application exception
    why_it_matters: a reader who asks for client errors and receives this feature gets policy violations and user-agent notices, not a stack trace from their own script
    separate: recorded under open.javascript_errors rather than folded in, because it needs a browser-side module and a different privacy argument
  reachable_here: csp-violation is the only type this framework configures a header for today; deprecation, intervention and crash arrive at the default endpoint with no header at all, and the remaining four need headers this framework does not set
  delivery_is_not_live: reports are batched and delayed by the browser, so this is a signal an operator reads afterwards, never a request-time one
browser_support:
  reporting_endpoints_header: Chrome and Edge 96, Firefox 130, Safari and iOS Safari 16.4
  csp_report_to_directive: Chrome 70, Edge 79, Firefox 149, Safari 16.4
  the_gap: Firefox 130 through 148 parses the header and does not honour the report-to directive, which is the entire case for still emitting report-uri
  chrome_only_types: deprecation, intervention and crash are Chromium notices; a cross-browser deployment should expect csp-violation and little else
transport_facts:
  request: POST, Content-Type application/reports+json, fetch mode cors, credentials mode same-origin
  credentials_consequence: a same-origin endpoint receives the visitor's cookies on a request the visitor never made and that carries no CSRF token, which is decision:report-endpoint-above-the-session
  endpoint_url: parsed against the response URL, so a relative path is legal and is what this framework emits
  secure_context: an endpoint whose origin is not potentially trustworthy is skipped, so plain-HTTP staging receives nothing and api:cli-dev on localhost receives everything
  failure_accounting: a non-2xx response increments the browser's failure counter for the endpoint and 410 Gone removes it, which is why this endpoint answers 2xx to a body it could not parse
decided_shape:
  header: Reporting-Endpoints with one entry named default, whose value is the configured path as a quoted structured-field string
  one_endpoint: default receives every type that names no endpoint of its own, so one name covers the whole surface and no report type needs a second key
  csp_wiring: append report-to default to the enforced and report-only policies, and append report-uri with the same path unless the deployment turns it off
  author_policy_untouched: a configured policy already naming report-to or report-uri is left exactly as written, because policy:security-response-headers gives the policy text to the application author
  path: an absolute path inside the reserved /_pw/ prefix, defaulting to /_pw/report, so serveReservedPath already closes it and no application route can collide
  slot: pwruntime SlotOperational, per decision:report-endpoint-above-the-session
  resolution_site: pwruntime, so ResolveSecurityHeaders emits one header set for both transports and the parse-and-redact core is called by each rather than written twice, per decision:shared-runtime-leaf
  record: data:browser-report-record
handler_contract:
  method: POST only; every other method answers 405 with an Allow header, since operationalMethod admits GET and HEAD and this endpoint admits neither
  content_type:
    application/reports+json: the Reporting API batch
    application/csp-report: the legacy report-uri body, accepted because report-uri is emitted
    anything_else: 415, which is safe because no browser delivery sends another type and a non-2xx here reaches nothing that counts failures
  body_bound: its own cap, defaulting to 64 KiB, rather than server.max_request_body, which is sized for uploads
  batch_bound: a cap on reports read from one body, defaulting to 32, with the remainder counted and dropped
  response: 204 with Cache-Control no-store, including for a body that failed to parse and for reports the rate bound dropped
  parse_failure: one bounded diagnostic record naming the length and the content type, never the body
flood_control:
  problem: one misconfigured directive on a popular page produces one report per page view, and the burst arrives during the incident whose log an operator is trying to read
  shape: a token bucket over records written, defaulting to 10 per second with a small burst, dropping past it and writing one dropped-count record per window
  drop_rather_than_queue: policy:log-emission keeps a log queue out of the response path deliberately, and requirement:live-error-report-off-lock records what a queued reporter costs
  http_unaffected: a dropped report is still answered 204, because refusing one would make a browser back off from an endpoint whose only problem is that it is working
  already_covered: SlotRateLimitProcess, SlotMaxRequestBody, SlotRecover and SlotAccessLog all sit above SlotOperational, so a configured process bound, the global body cap, panic recovery and the access log reach this endpoint without anything new
configuration:
  binding: data:security-runtime-config security.reporting
  dependency: reporting enabled with headers.enabled false emits no header and is a startup error, per the data:middleware-runtime-config rule that disabling a dependency of an enabled feature is one
  validation: absolute path, inside the reserved prefix, no quote or control character, and no collision with health, readiness, openapi or the public mount
must:
  - strip query and fragment from every URL-shaped value before it reaches a record, because a document URL or a blocked URL can carry a token
  - reduce a data: or blob: blocked URL to its scheme, since the payload is the injected content itself
  - cap every string attribute at a fixed byte length, because every field on this endpoint is written by whoever posts to it
  - rely on policy:log-emission control-character escaping rather than adding a second one, and state the dependency
  - never record original_policy, which echoes the deployment's own configured policy at every violation and says nothing the configuration does not
  - never record user_agent, which is high cardinality and which the access log already holds for the same request
  - keep the raw body out of every record, including the parse-failure diagnostic
default_state:
  decided: on whenever headers.enabled is on, 2026-08-10
  why: the argument that put DefaultContentSecurityPolicy in the tree applies unchanged; a policy whose breakage nobody can see is the failure this feature exists to fix, and a default that has to be discovered is one most affected projects will not find
  accepted_cost: an unauthenticated POST endpoint on every deployment, which is what the body, batch, and rate bounds above are for
  contrast: requirement:network-error-logging defaults off instead, because its policy persists in the browser and this one does not
open:
  javascript_errors:
    what: window.onerror and unhandledrejection reported by the framework's own browser runtime to this same path
    fits: the runtime module already ships and the endpoint, the bounds and the record shape would all be reused
    costs: a distinct payload dialect, a stack trace that is only readable against requirement:deployed-debug-information source maps, and a privacy argument about reporting every visitor's script failures that CSP violations did not need
    stance: a separate requirement, not a widening of this one
  external_collector: a configured absolute URL that replaces the framework path in the header, registering no local handler; nearly free, and it removes the logging that is this request's whole point
  dev_console: routing reports into requirement:dev-telemetry-viewer as well as the log, which is the shape policy:log-emission already carves out for api:cli-dev
  doctor_check: whether rule:production-readiness-checks should report an enforcing CSP with no reporting endpoint, and a reporting endpoint on a deployment serving plain HTTP, since neither can produce a report
non_goals:
  - JavaScript runtime errors, per open.javascript_errors
  - the NEL header and its network-error reports, which are requirement:network-error-logging and share this endpoint without widening this requirement
  - COOP, COEP, Permissions-Policy and Document-Policy reporting, none of whose headers this framework sets
  - the deprecated Report-To header, which no supported browser still needs
  - storing, aggregating, deduplicating or displaying reports; the log is the product, as the request stated
  - answering a report with anything an application handler can observe
acceptance:
  - a default project serves a response carrying Reporting-Endpoints and a policy naming report-to default
  - a CSP violation on a Chrome, Firefox and Safari page each produces one warn record naming the effective directive and the blocked URL
  - a POST of unparseable bytes is answered 204 and produces exactly one bounded diagnostic
  - a 64 KiB body limit and a 32-report batch limit are each enforced, and the excess is counted rather than logged
  - a burst past the rate bound writes one dropped-count record and still answers 204
  - a request to the endpoint reaches no session, no authentication and no CSRF check
  - the endpoint path 404s through serveReservedPath when reporting is disabled
  - a document URL carrying a query string reaches the log without it
  - both transports emit byte-identical headers and write identical records for one body
  - the website security-headers guide no longer shows report-uri pointing at an unserved path
```
