---
id: decision:dynamodb-observability-seam
type: decision
title: Observe DynamoDB At The HTTP Client
---
Popcorn Web instruments the DynamoDB path by supplying the driver's HTTP client, because there is no executor seam the way api:instrumented-sql-executor has one.

```yaml
status: accepted
problem:
  sql: generated SQL resolves an executor from the request context, so api:instrumented-sql-executor decorates it without changing generated output
  dynamo: generated dynamobind code calls the driver with a *dynamodb.Client the caller passed, so there is nothing between the call and the wire to decorate
seam:
  option: WithHTTPClient on the driver constructor, per system:tinygodriver-dynamodb
  owner: api:dynamo-package, at startup
  form: a http.RoundTripper wrapping the transport the driver would otherwise build
observable_at_that_layer:
  - the operation, from the X-Amz-Target header
  - the table, from the request body
  - status, latency, retry count, and the x-amzn-RequestId of the reply
  - consumed capacity, when the request asked for it
  not_observable: the Go call site and the application type, both of which are gone by the time the request is built
consequence:
  close: the driver leaves a caller-supplied HTTP client alone on Close, so api:dynamo-package owns closing the transport it built
  pool: the max_idle_conns setting of data:dynamodb-runtime-config is applied to the supplied client, not by the driver
alignment:
  query_diagnostics: the record shape follows data:query-record where it fits, so one viewer shows both stores
  safety: policy:query-log-safety governs it; a key value and an expression value are data and are not logged
framework_owned_tables_are_excluded:
  rule: policy:query-log-safety, which records nothing for framework storage traffic in any store
  why_it_matters_more_here: the reproduction value of decision:dynamodb-framework-scope comes from the captured body being the exact request, so for requirement:dynamodb-session-store an included record would carry the key hash, the CSRF secret, and the payload verbatim
  where_the_filter_sits: at this seam, on the table the request names, before it is sent
  otel: the span is a client span with the operation and table as attributes, per requirement:modern-observability
enabled_by: the same observability configuration that enables SQL query diagnostics, so an operator turns on one switch
rejected_alternative:
  form: a pw wrapper around every dynamobind call that records before delegating
  why_not: decision:dynamodb-no-runtime-abstraction refuses exactly that wrapper, and it would still miss a retry the driver performs internally
related:
  - api:dynamo-package
  - api:instrumented-sql-executor
  - requirement:query-diagnostics
```
