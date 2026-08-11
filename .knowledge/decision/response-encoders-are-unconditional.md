---
id: decision:response-encoders-are-unconditional
type: decision
title: Every Build Carries Both Response Encoders
---
pw_nozstd and pw_nogzip are removed. Both response encoders are compiled into every build, and middleware.compression is the only switch.

```yaml
status: accepted
decided: 2026-08-11 by the author
supersedes_the_tag_half_of: decision:response-content-codings
what_the_tags_were_for: binary size on a target that terminates compression in front of the application, where an encoder is linked and never called
why_they_stopped_paying:
  then: the zstd encoder meant linking klauspost, whose decoder is roughly ten times the encoder and which nothing in a server calls
  now: tinygodriver v1.2.4 encodes through its own compress/zstd, so the decoder is not in the graph at all
  measured: 387 KB for the pair, stripped, against a 9.87 MiB helloworld — under four percent
  judgement: smaller than the question, and a build axis costs more than that to keep honest
what_replaces_them: middleware.compression false, which stops the encoders running and is the same saving at run time without forking the build
what_this_removed:
  files: pw/response_zstd_off.go and pw/response_gzip_off.go
  constants: zstdResponseSupported and gzipResponseSupported, both of which could only be true afterwards
  branches: the len(availableResponseCodings) == 0 guards in prepareResponseEncoder and reportCompressionCodings, unreachable once no tag can empty the list
what_survived_and_why:
  unavailable_coding_report: reportCompressionCodings still warns, because a configuration naming brotli still names a coding this framework does not encode; brotli is absent on purpose, per decision:response-content-codings
  passing_a_retired_tag: not an error, and selects nothing
tinygo_consequence: decision:tinygo-fasthttp-needs-nozstd, whose whole failure mode is the same klauspost decoder, was resolved by the same upstream release
```
