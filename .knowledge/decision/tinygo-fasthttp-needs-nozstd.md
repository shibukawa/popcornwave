---
id: decision:tinygo-fasthttp-needs-nozstd
type: decision
title: TinyGo Builds fasthttp, And Briefly Needed fasthttp_nozstd
---
TinyGo compiles the fasthttp second build with no extra tag, since tinygodriver v1.2.4. For one day it needed `-tags fasthttp_nozstd`, and before that it was recorded as not linking at all, which was wrong.

```yaml
status: resolved
resolved_by:
  release: tinygodriver v1.2.4, "fasthttp: encode zstd through compress/zstd under TinyGo"
  effect: a TinyGo fasthttp build needs no extra tag at all
  measured: 2026-08-11, examples/helloworld, darwin/arm64 — 5.59 MiB with -tags fasthttp alone, and the binary serves
  fasthttp_nozstd_now: still accepted, saves about 40 KB, and there is little reason to pass it
  everything_below: describes the state between the two corrections, kept because the mechanism is what the reference page explains
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
  why_the_gc_escape_does_not_fire:
    fact: TinyGo sets the gc build tag, verified by compiling a file pair guarded on gc and !gc under both compilers
    effect: klauspost's pure-Go half is "(!amd64 && !arm64) || appengine || !gc || noasm", so the escape written for other compilers never selects under TinyGo and only noasm is left
  why_net_http_escapes:
    both_symbols_are_decoder_side: buildDtable_asm and sequenceDecs_decodeSync_safe_arm64 decode; nothing in the encoder is missing
    pw_only_encodes: a server compresses responses, so TinyGo's dead-code elimination drops the decoder and the assembly with it
    pw_already_swaps_the_package: pw/response_zstd_tinygo.go is built under "(tinygo || force_tinygo_logic) && !pw_nozstd" and uses tinygodriver/compress/zstd, while pw/response_zstd_std.go uses klauspost
    the_fork_decodes_too: BodyUnzstd, writeUnzstd and the FS pre-compressed .zst read reach the decoder, which is what retains the assembly
  therefore: this is not a fasthttp problem or a size problem; it is the decode half of one dependency reached under a compiler that cannot link it
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
    cannot_be_made_automatic_by_a_library: build tags are set by the build invocation, so no module can inject one into a dependency it imports; only klauspost adding !tinygo to the asm constraint, or the build tool passing the flag, can make TinyGo imply it
    right_layer_instead: the importer declines the package under tinygo, per requirement:tinygodriver-encode-only-zstd
verified_running: the fasthttp_nozstd binary serves, logs transport=fasthttp, and answers a request through the middleware chain; a client offering only zstd is served identity rather than failing
open_work_on_this_side:
  build_command: closed — there is no tag left to pass
  compression_default: closed — a TinyGo fasthttp build produces zstd again, so the "zstd,gzip" default is honest on every target
  scheduler_threads:
    fact: a TinyGo build linking database/postgres or database/mysql needs -scheduler=threads, and refuses to compile without it
    why: under the cooperative scheduler a blocking socket call holds the runtime, so the driver's cancellation watcher never runs and a query outlives its deadline returning nil
    how_it_is_passed: the Dockerfile.tinygo written by api:cli-init; pw build does not drive TinyGo
  ipv4_only: a TinyGo binary binds IPv4 while a Go build binds the IPv6 wildcard, which is a probing trap and not a defect
tinygodriver_additions: requirement:tinygodriver-encode-only-zstd
```
