---
id: requirement:response-gzip-encoder
type: requirement
title: Gzip Response Encoder
---
The gzip half of policy:response-content-encoding selects an optimized host encoder or the standard library one behind a single response-writer interface, mirroring how requirement:contrib-zstd splits.

```yaml
implementation_selection:
  host_go:
    condition: "!tinygo && !force_tinygo_logic"
    backend: github.com/klauspost/compress/gzip
    level: 1
    already_a_dependency: true, through the host backend of requirement:contrib-zstd, so the coding adds 148 KB and no module
  tinygo:
    condition: "tinygo || force_tinygo_logic"
    backend: compress/gzip
    level: 1
    reason: pure Go with no assembly path to validate, and it compiles for wasip1 today
  force_policy: decision:force-tinygo-logic
level_is_1_because:
  - a dynamic body is encoded while a request waits, so throughput is the scarce resource and ratio is the one that can give
  - level 1 is the fastest level that still compresses, at 464 MB/s on the host and 309 on TinyGo against 60.9 percent for Huffman-only
  - it costs 3.3 points of ratio against level 6 and returns a third to two thirds more throughput, per the gzip_level_curve of decision:response-content-codings
  - on the standard library it is not a judgment call: only level 1 uses the fast encoder, and level 2 is already half the speed for 1.7 points
  - one level on both backends means a response does not change size when a target changes, so a test asserting encoded bytes holds across builds
  - the deep levels are not lost; policy:public-asset-precompression takes them where the cost lands on the build
why_not_one_backend:
  stdlib_on_the_host: 1.5 times slower than klauspost at level 1, per decision:response-content-codings
  klauspost_on_tinygo: carries architecture-specific assembly with pure-Go fallbacks, so it is a validation cost for a target where the standard library already works
required_of_both:
  - construction writes nothing to the destination, per the lazy_frame_header rule of requirement:contrib-zstd; both gzip backends defer their header to the first Write already
  - Write, Flush, Close, and an Abort that discards an uncommitted frame
  - Flush emits everything written so far as complete blocks and leaves the stream open, which is the compress/flate sync-flush shape requirement:contrib-zstd was already specified against
  - Flush does not flush the destination writer; the caller chains that
  - Flush after Close is an error
  - identical public lifecycle and errors across backends, so the negotiation path branches on neither
pooling:
  host: reset and reuse through sync.Pool, as the zstd encoder already does, which makes steady-state per-request allocation zero
  tinygo: an encoder is allocated per response; whether pooling helps depends on the target allocator and is not assumed
shared_shape:
  - one interface covers zstd and gzip, so policy:response-content-encoding holds a coding token and a constructor rather than a type per coding
  - a build lacking a coding reports it as unsupported instead of failing to compile the negotiation
build_tag:
  name: pw_nogzip
  removes: the gzip encoder alone, as pw_nozstd removes the zstd one
  why_a_tag_per_coding: pw_nozstd keeps meaning what it always did, so an existing build line is not silently redefined, and a target wanting neither passes both
  both_tags: availableResponseCodings is empty, so middleware.compression is a setting with no effect and startup says so
open_questions:
  - the per-writer memory of the TinyGo encoder, which level 1 settles on speed but not on footprint; a constrained target may still want a smaller window than the standard library's fixed one, and that is a measurement nobody has taken
references:
  - https://www.rfc-editor.org/rfc/rfc1952
```
