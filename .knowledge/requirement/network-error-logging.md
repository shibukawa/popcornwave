---
id: requirement:network-error-logging
type: requirement
title: Network Error Logging
---
A deployment may emit an `NEL` header so browsers report connection failures against its origin to the requirement:browser-report-ingest endpoint, which is the only signal here describing requests this server never received.

```yaml
status: requirements recorded 2026-08-10, unimplemented
priority: could
source: user decision 2026-08-10, taken as a sibling of requirement:browser-report-ingest rather than folded into it
sibling_not_extension:
  shares: the endpoint, the bounds, the record shape, the severity map, and the always-2xx contract
  differs: its own response header, its own persisted browser-side policy, its own sampling, and a report body describing a request rather than a document
  therefore: one requirement each, because the parts that differ are the parts that carry the risk
what_it_buys:
  unique: every other report type describes something that happened inside a page this server served; a network error describes a request that never arrived, so nothing in the access log, the trace, or the span set can see it
  cases: DNS failure, TCP reset or timeout, TLS certificate rejection, protocol error, and a response an intermediary produced
  phases: dns, connection, application
  types: dns.unreachable, dns.name_not_resolved, tcp.timed_out, tcp.reset, tls.cert.invalid, http.error, http.protocol.error, abandoned, ok
browser_support:
  verified: 2026-08-10 against caniuse and the W3C Network Error Logging specification
  supported: Chrome 71, Edge 79
  never: Firefox and Safari, on every version and every platform
  usage: about 76 percent globally, all of it Chromium
  consequence: this is a Chromium-only signal and must be read as one; an outage invisible to it may still be an outage
the_honest_limitation:
  problem: the reports describe a failure to reach this origin, and the endpoint they are delivered to is on this origin
  spec_position: the specification recommends an endpoint on an origin whose infrastructure is not coupled to the reporting one
  what_actually_arrives: reports are queued client-side and delivery is retried, so a share of an outage's reports arrive after the outage from clients that came back, carrying a large report.age_ms; nothing arrives from a client that did not return
  therefore: this reports intermittent and client-side failures well and a total outage poorly, and a deployment wanting the second needs an endpoint this framework does not host
  not_a_reason_to_skip_it: certificate errors, protocol errors, and per-client connection failures are the common case and reach a same-origin endpoint fine
policy_is_persisted:
  mechanism: the user agent keeps a policy cache keyed by origin and network partition key, and max_age is its lifetime
  hazard: this is the HSTS shape; a wrong value stays in browsers for its lifetime and no deployment can withdraw it early
  precedent_here: data:security-runtime-config already defaults headers.hsts.enabled to false for the same reason, and this default follows it
  withdrawal: a max_age of zero clears the policy, and only reaches a browser that makes another successful request
secure_origin_only:
  rule: policy registration aborts unless the origin is potentially trustworthy
  consequence: plain-HTTP staging registers nothing, and api:cli-dev over http on localhost registers a policy for localhost, which is not the origin anybody wants reports about
  therefore: unlike requirement:browser-report-ingest, this has no useful development mode and is a deployment-only feature
decided_shape:
  header: NEL, whose value is a JSON object naming report_to default so it resolves through the one Reporting-Endpoints entry already emitted
  emission: beside the other managed headers of policy:security-response-headers, resolved once by ResolveSecurityHeaders like everything else there
  default: off, per policy_is_persisted
  sampling:
    failure_fraction: 1 by default, because a failure is the signal
    success_fraction: 0 by default, and a positive value samples ok reports from every successful request, which is a copy of the access log arriving over the network and paid for twice
    guard: a success_fraction above a small ceiling is a startup error rather than a configuration a deployment discovers through its log bill
  record: data:browser-report-record gains the network-error fields; the severity map reads a network error as warn, since a failed request is not a page that merely warned
configuration:
  binding: data:security-runtime-config security.reporting.nel
  fields: enabled, max_age, include_subdomains, success_fraction, failure_fraction
  dependency: nel.enabled with reporting.enabled false is a startup error, because report_to would name an endpoint no header declares
must:
  - drop request_headers and response_headers, which the specification allows a policy to request and which would put caller headers into a log record
  - record server_ip only when it is this deployment's own, since it is otherwise infrastructure detail in a record anyone can post
  - keep referrer under the same query-stripping data:browser-report-record applies to every URL-shaped value
  - treat a dns.address_changed report as the downgraded form it is, and never read its absent fields as zero
  - state max_age in the configuration as a duration and emit it as seconds, matching how headers.hsts.max_age already reads
open:
  alternate_origin: whether to support a configured absolute URL for this one header only, so a deployment can send network errors somewhere uncoupled while CSP violations stay local; it is the fix for the_honest_limitation and it is also a second endpoint shape to validate
  doctor_check: whether rule:production-readiness-checks should report an enabled NEL policy on a deployment serving plain HTTP, which registers nothing
non_goals:
  - hosting the uncoupled endpoint the specification recommends
  - alerting, aggregation, or availability measurement built on these reports
  - the deprecated Report-To header, which requirement:browser-report-ingest already declines
acceptance:
  - a deployment with nel.enabled false emits no NEL header and registers no policy
  - an enabled deployment over HTTPS emits one NEL header naming report_to default
  - a network error report is written as one warn record naming the phase, the type, and the elapsed time
  - request_headers and response_headers never appear in a record
  - success_fraction above the configured ceiling fails startup
  - a policy withdrawal is expressible, and max_age zero is emitted rather than the header being dropped
```
