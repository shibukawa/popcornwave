---
id: requirement:tinygodriver-encode-only-zstd
type: requirement
title: What The fasthttp Build Would Ask Of tinygodriver
---
Nothing tinygodriver ships is missing for the fasthttp build to work under TinyGo today. One addition would pay: an encode-only zstd, so a server emits zstd for 0.08 MB instead of paying 2.40 MB or dropping the codec.

```yaml
status: proposed
serves: decision:tinygo-fasthttp-needs-nozstd
scope: this concept records what popcornwave would ask of tinygodriver; the work and the decision are tinygodriver's
already_provided_nothing_to_add:
  fasthttp_fork: a drop-in fasthttp whose patches are recorded in fasthttp/PATCHES.md
  fasthttp_nozstd: the tag that makes the TinyGo build link, already present and already measured there
  fasthttprouter: vendored, and popcornwave already imports it
  fasthttpwebsocket: a fasthttp/websocket fork whose server half needed no patches, which is the library requirement:contrib-websocket would build on when that work starts
the_one_addition:
  name: encode-only zstd for the fasthttp fork
  today_the_choice_is_binary:
    klauspost: 2.40 MB of TinyGo binary, encode and decode
    fasthttp_nozstd: 0 MB, no zstd at all, and a client offering only zstd is served identity
  proposed_middle: tinygodriver's own compress/zstd encoder at 0.08 MB, encode only
  why_it_fits_a_web_framework: a server compresses responses and rarely reads a compressed request body, so the decode half is what popcornwave pays for and does not use
  ratio_is_no_longer_the_objection: that encoder now fits FSE tables per block and codes literals, landing at 8.2% and 11.0% against deflate's 11.6% and 13.3%
  what_blocks_it:
    - compress/zstd excludes decoding, so BodyUnzstd, AppendUnzstdBytes and the FS pre-compressed .zst read have nothing to call
    - it offers no Reset and no compression levels, which are what fasthttp's per-level encoder pools are built on
  recorded_upstream: tinygodriver fasthttp/PATCHES.md already names this as coherent to want and explicitly not what the tag does today, so this is a shared conclusion rather than a new proposal
smaller_asks:
  discoverability: the tag is documented in PATCHES.md, which is not where someone building a popcornwave app looks; naming it in the fasthttp README's TinyGo section would have saved the wrong conclusion recorded in decision:tinygo-fasthttp-needs-nozstd
  net_http_import: tinygodriver/fasthttp imports net/http, so a fasthttp binary still links it; whether that is separable is unmeasured here and may be load-bearing for the fork
acceptance:
  - a TinyGo fasthttp build can advertise zstd without linking klauspost
  - or the framework narrows its compression default and the ask is withdrawn
```
