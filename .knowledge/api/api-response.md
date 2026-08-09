---
id: api:api-response
type: api
title: API Response API
---
pw.WriteAPI owns content negotiation, optimized serialization, compression, and structured error reporting for typed API values.

```yaml
surface:
  - WriteAPI(http.ResponseWriter, *http.Request, T)
behavior:
  - negotiate an accepted representation
  - set response headers
  - apply configured compression, per policy:response-content-encoding, by wrapping the response writer before the registered writer sets its own type and status
  - keep the wrapper chain walkable through Unwrap, so commit detection still reaches the writer underneath
  - discard an uncommitted frame and take the Content-Encoding header back off when serialization fails, so the api:problem-response body replacing it is not labelled with a coding it is not in
  - use generated optimized JSON codecs
  - log and trace serialization failures
  - use api:problem-response for safe errors when possible
low_level:
  - pw does not re-export EncodeJSON or DecodeJSON
  - applications needing raw codecs may intentionally import TinyBind jsonbind
```
