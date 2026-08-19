---
id: requirement:contrib-cbor
type: requirement
title: TinyGo CBOR and COSE Foundation
---
Bounded reflection-free CBOR decoding and deterministic encoding for WebAuthn authenticator and COSE key data, supplied by system:tinygodriver encoding/cbor since v1.2.6; the contrib/cbor implementation was upstreamed there and removed here.

```yaml
package: github.com/shibukawa/tinygodriver/encoding/cbor
history: implemented as contrib/cbor first, upstreamed into tinygodriver v1.2.6, contrib/cbor deleted
public_api:
  - NewDecoder(io.Reader, DecoderOptions)
  - typed token and container readers
  - RawMessage for bounded deferred decoding
  - NewEncoder(io.Writer, EncoderOptions)
upstream_additions:
  - NewReader byte-slice reader with typed width-checked readers
  - Marshal* single-item helpers
  - selectable KeyOrder (length-first CTAP2/COSE default, bytewise core deterministic)
  - profile presets and RejectFloats
supported:
  - unsigned and negative integers
  - byte and text strings with UTF-8 validation
  - arrays and maps
  - booleans, null, tags, and floating-point values
  - definite and indefinite-length input
  - Core Deterministic Encoding output
required_behavior:
  - reject malformed additional information and truncated values
  - reject duplicate map keys in security-sensitive decode mode
  - preserve signed integer map labels required by COSE_Key
  - consume exactly one root item unless sequence mode is explicit
  - avoid io.ReadAll and decode incrementally from io.Reader
limits:
  - input bytes
  - nesting depth
  - container items
  - string and byte-string length
  - retained RawMessage bytes
non_goals:
  - reflection-based arbitrary struct mapping
  - COSE signing, encryption, or key management beyond passkey parsing needs
standards:
  cbor: https://www.rfc-editor.org/rfc/rfc8949.html
  cose_structure: https://www.rfc-editor.org/rfc/rfc9052.html
  cose_algorithms: https://www.rfc-editor.org/rfc/rfc9053.html
```
