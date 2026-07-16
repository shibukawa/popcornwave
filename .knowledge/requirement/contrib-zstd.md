---
id: requirement:contrib-zstd
type: requirement
title: Zstandard Response Encoder
---
contrib/zstd selects an optimized host encoder or bounded TinyGo encoder while preserving one response and cache-identity API.

```yaml
package: contrib/zstd
public_api:
  - NewWriter(io.Writer, options) returns Writer
  - Writer.Result returns encoded size and SHA-256 after successful Close
  - EncodeAll(src, options) returns encoded bytes and Result
  - WithETag(bool) controls cache hash generation; default true
  - Result.ETag returns a quoted strong HTTP entity-tag or empty when disabled
etag_disabled:
  - do not allocate or update SHA-256
  - Result.ETagEnabled is false
  - Result.SHA256 is zero and Result.ETag is empty
  - use for Cache-Control no-store responses
implementation_selection:
  host_go:
    condition: "!tinygo && !force_tinygo_logic"
    backend: github.com/klauspost/compress/zstd
    settings: default level, concurrency 1, 128 KiB window, lower memory, frame checksum disabled
  petitweb:
    condition: "tinygo || force_tinygo_logic"
    backend: bounded internal encoder
  force_policy: decision:force-tinygo-logic
invariants:
  - public API and lifecycle errors match across backends
  - enabled SHA-256 and ETag cover the backend's emitted encoded representation
encoder_required:
  - standard frames
  - bounded raw and RLE blocks
  - low-memory single-match compressed blocks with RLE sequence tables
  - enabled SHA-256 updated only for bytes successfully emitted
limits:
  petitweb: 128 KiB window and retained input block plus 16 KiB match table
deferred:
  - Huffman literals, general FSE tables, multi-match blocks, and additional compression levels
  - trained dictionaries
  - seekable format
  - assembly optimizations
  - decoding
standard: https://www.rfc-editor.org/rfc/rfc8878
```
