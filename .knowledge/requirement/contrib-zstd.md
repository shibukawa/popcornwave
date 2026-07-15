---
id: requirement:contrib-zstd
type: requirement
title: TinyGo Zstandard Codec
---
contrib/zstd provides bounded pure-Go Zstandard streaming decode and basic encode APIs interoperable with RFC 8878 implementations.

```yaml
package: contrib/zstd
public_api:
  - NewReader(io.Reader, options) returns io.ReadCloser
  - DecodeAll(dst, src) returns bytes
  - NewWriter(io.Writer, options) returns WriteCloser
  - EncodeAll(dst, src) returns bytes
decoder_required:
  - standard frames
  - raw, RLE, and compressed blocks
  - concatenated frames
  - skippable frames
  - optional content checksum verification
  - explicit unsupported-window and dictionary errors
encoder_required:
  - standard frames
  - raw and RLE blocks
  - one low-memory compressed level after interoperability vectors pass
limits:
  - maximum window size
  - maximum output size for DecodeAll
  - maximum block and table allocation
  - no allocation proportional to declared content size before data arrives
deferred:
  - trained dictionaries
  - seekable format
  - assembly optimizations
  - parity with all upstream compression levels
standard: https://www.rfc-editor.org/rfc/rfc8878
```
