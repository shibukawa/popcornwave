---
id: requirement:api-lifecycle-response-headers
type: requirement
title: API Lifecycle Response Headers
---
API resources can announce deprecation and expected shutdown without changing current behavior.

```yaml
standards:
  deprecation: RFC 9745, https://www.rfc-editor.org/rfc/rfc9745
  sunset: RFC 8594, https://www.rfc-editor.org/rfc/rfc8594
response:
  Deprecation: RFC 9651 Structured Field Date, serialized as @ followed by Unix seconds
  Sunset: HTTP-date in IMF-fixdate form
  Link:
    deprecation: optional HTTPS migration or policy document with rel="deprecation"
    sunset: optional HTTPS shutdown or mitigation document with rel="sunset"
rules:
  - each header applies to the response resource unless linked documentation defines a wider API scope
  - deprecation is a lifecycle hint and does not change status, semantics, or availability
  - sunset means expected unavailability, not merely loss of preferred status
  - when both dates exist, sunset is not earlier than deprecation
  - emit configured fields on success and error responses before commitment
  - validate dates, ordering, link targets, and field values at registration or startup
surface:
  - pw.Lifecycle carries DeprecatedAt, SunsetAt, and DocumentationURL
  - pw.LifecycleHeaders validates once and returns route-scoped middleware
  - the middleware serializes Deprecation, Sunset, and Link consistently
documentation: requirement:web-standards-overview
acceptance:
  - exact wire-format tests cover future and past deprecation dates
  - invalid ordering or header injection fails before serving
  - unchanged resource behavior is verified while lifecycle headers are present
```
