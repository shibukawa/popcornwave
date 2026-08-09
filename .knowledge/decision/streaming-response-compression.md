---
id: decision:streaming-response-compression
type: decision
title: Streaming Response Compression
---
The streaming branch of decision:automatic-async-render-selection negotiates content encoding like any other HTML response, because the encoder can now flush a block without ending the frame.

```yaml
status: accepted
context:
  - an encoder without flush holds completions until Close, which defeats progressive delivery
  - htmlbind.Flush discovers Flush by interface assertion, so a non-flushing wrapper degrades silently
  - requirement:contrib-zstd gained Writer.Flush in system:tinygodriver v1.0.4 on both backends
decision:
  - negotiate Accept-Encoding on the streaming branch exactly as the buffered branch does
  - pw wraps the encoder in a writer whose Flush flushes the encoder and then the http.Flusher, so one htmlbind.Flush reaches both
  - flush once per settled boundary, never per Write, because neither dynamic encoder flushes its destination
  - the streaming branch still omits Content-Length and still adds Vary Accept-Encoding
  - keep ETag generation off, as api:html-response already does, since Result is only readable after Close
coding_neutral:
  - this decision is about when to flush, not about which coding was selected; policy:response-content-encoding owns the coding set and its order
  - every offered coding must supply the sync-flush shape, which is why requirement:response-gzip-encoder demands it of both gzip backends
consequences:
  - a per-boundary flush emits a short block, so a streamed page compresses worse than the same page buffered, for every coding
  - flushing per boundary makes compressed length a finer-grained oracle; do not stream secret-bearing boundaries alongside attacker-controlled input
  - a proxy that buffers encoded responses can be worked around by forcing the buffered branch through data:html-render-config
verification: a streamed compressed response decodes to the same framing, and one flush per boundary reaches the response writer
```
