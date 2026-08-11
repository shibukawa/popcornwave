---
id: decision:tinygo-fasthttp-needs-nozstd
type: decision
title: TinyGo Builds fasthttp Only With fasthttp_nozstd
---
TinyGo compiles the fasthttp second build today. It needs `-tags fasthttp_nozstd` alongside `fasthttp`, and the earlier record that TinyGo could not link fasthttp was wrong.

```yaml
status: accepted
supersedes_claim: "tinygo + fasthttp does not link", recorded 2026-08-11 during the binary size measurement and corrected the same day
serves: requirement:second-build-feature-parity
measured: 2026-08-11, examples/helloworld, darwin/arm64, Apple M3, klauspost/compress v1.19.1
failure_without_the_tag:
  symptom: linker could not find symbol
  symbols:
    - zstd.buildDtable_asm from zstd/fse_decoder_asm.go
    - zstd.sequenceDecs_decodeSync_safe_arm64 from zstd/seqdec_arm64.go
  cause: hand-written arm64 assembly TinyGo's linker does not resolve, selected by the constraint "(amd64 || arm64) && !appengine && !noasm && gc"
  importer: tinygodriver/fasthttp imports klauspost/compress/zstd unconditionally
  why_net_http_escapes: pw imports zstd too and TinyGo links it, so the assembly is reached only on the fasthttp path
sizes:
  tinygo_net_http: 4.19 MiB
  tinygo_fasthttp_nozstd: 5.55 MiB
  tinygo_fasthttp_noasm: 8.04 MiB
  no_debug: changes nothing measurable in any of the three
tags:
  fasthttp_nozstd:
    effect: drops the zstd dependency; tinygodriver's own tag, documented in its fasthttp/PATCHES.md section 6
    correct_choice: yes
  noasm:
    effect: klauspost's tag, swaps the assembly for its pure-Go fallback and keeps zstd
    correct_choice: no, it links but pays 2.49 MiB for a codec the build then still advertises
verified_running: the fasthttp_nozstd binary serves, logs transport=fasthttp, and answers a request through the middleware chain; a client offering only zstd is served identity rather than failing
open_work_on_this_side:
  build_command: api:cli-build does not pass fasthttp_nozstd for a TinyGo fasthttp target, so the documented failure is what a user meets first
  compression_default: pwconfig CompressionCodings defaults to "zstd,gzip" with zstd first, and a nozstd build cannot produce it; whether the framework should narrow the default, detect the tag, or warn at startup is undecided
  ipv4_only: a TinyGo binary binds IPv4 while a Go build binds the IPv6 wildcard, which is a probing trap and not a defect
tinygodriver_additions: requirement:tinygodriver-encode-only-zstd
```
