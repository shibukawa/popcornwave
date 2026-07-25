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
  - apply configured compression
  - use generated optimized JSON codecs
  - log and trace serialization failures
  - use api:problem-response for safe errors when possible
low_level:
  - pw does not re-export EncodeJSON or DecodeJSON
  - applications needing raw codecs may intentionally import TinyBind jsonbind
```
