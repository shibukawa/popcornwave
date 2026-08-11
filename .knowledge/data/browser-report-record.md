---
id: data:browser-report-record
type: data
title: Browser Report Record
---
One report delivered to requirement:browser-report-ingest is normalized from either payload dialect into one bounded record before api:logger writes it.

```yaml
input_dialects:
  reporting_api:
    content_type: application/reports+json
    shape: a JSON array of objects carrying age, type, url, user_agent and body
    csp_body_keys: documentURL, referrer, blockedURL, effectiveDirective, originalPolicy, disposition, statusCode, sourceFile, sample, lineNumber, columnNumber
  legacy_report_uri:
    content_type: application/csp-report
    shape: one object under a csp-report key
    keys: document-uri, referrer, blocked-uri, violated-directive, effective-directive, original-policy, disposition, status-code, source-file, script-sample, line-number, column-number
  why_both: the report-uri directive is still emitted for the Firefox range that parses Reporting-Endpoints without honouring report-to, so both dialects arrive at one path
  normalization: the legacy body is read as a csp-violation whose age is zero, so one record shape covers both and nothing downstream asks which dialect arrived
severity:
  crash: error
  network-error: warn
  csp-violation: warn
  integrity-violation: warn
  coep: warn
  coop: warn
  permissions-policy-violation: warn
  deprecation: info
  intervention: info
  unrecognized: info
  rationale: a blocked resource is a page that did not work and a notice is a page that will stop working, which are different readings and should not share a level
message: browser report
attributes:
  report.type: the reported type, truncated
  report.url: document URL, origin and path only
  report.age_ms: int64, zero for the legacy dialect
  report.message: the notice text of a non-CSP type, truncated
  report.source_file: script URL, origin and path only
  report.line: int64
  report.column: int64
  report.id: the deprecation or intervention identifier, truncated
  csp.effective_directive: the directive that matched
  csp.blocked_url: origin and path, or the scheme alone for an opaque scheme
  csp.disposition: enforce or report
  csp.sample: the violating text the browser sampled, already capped at 40 characters by the CSP specification
  net.phase: dns, connection, or application, from requirement:network-error-logging
  net.error_type: the reported failure, such as dns.name_not_resolved or tls.cert.invalid
  net.elapsed_ms: int64
  net.method: the request method
  net.protocol: the ALPN protocol name
  net.status_code: int64
  net.sampling_fraction: float64, without which a count of these records means nothing
  net.server_ip: recorded only when it is this deployment's own address
never_recorded:
  request_headers_and_response_headers: an NEL policy may ask a browser to include them, and this framework never does, because they carry caller headers into a log record
  original_policy: echoes the deployment's own configured policy on every violation and adds nothing the configuration does not already state
  user_agent: high cardinality, and the access log holds it for the same request
  referrer: report.url already names the page that broke, and a referrer can carry a query nobody meant to log
  raw_body: including in the parse-failure diagnostic, which names the length and the content type only
bounds:
  string_attribute: a fixed byte cap per value, because every field here is written by whoever posts to the endpoint
  url_shaped: query and fragment stripped before truncation
  opaque_scheme: a data:, blob: or filesystem: URL is reduced to its scheme, since the rest of it is the injected payload
  numeric: a non-numeric line or column is dropped rather than coerced
  escaping: policy:log-emission escapes control characters and line breaks, and this record adds no second implementation of that
rules:
  - every attribute follows data:log-attribute, so values stay scalar and no reserved record field can be replaced
  - an unrecognized report type is recorded rather than refused, because the type set grows without this framework changing
  - a report missing the fields its type declares is still recorded, with the absent attributes omitted rather than emitted empty
  - policy:query-log-safety is untouched here; nothing on this path reaches a database
```
