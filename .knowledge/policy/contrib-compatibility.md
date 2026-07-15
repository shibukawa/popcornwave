---
id: policy:contrib-compatibility
type: policy
title: Contrib Compatibility Policy
---
Every contrib package must remain small, explicit about compatibility, and continuously verified on host Go and supported TinyGo targets.

```yaml
requirements:
  - public API documents supported and unsupported behavior
  - runtime code avoids reflection-driven field or method discovery
  - avoid unsafe, assembly, and CGo unless a package-specific decision permits them
  - bound attacker-controlled allocation and recursion
  - accept context.Context for blocking network or database operations
  - expose deterministic shutdown and resource release
  - host Go unit tests use upstream vectors when available
  - api:cli-check compiles each imported package with TinyGo
matrix:
  required:
    - host Go linux amd64
    - TinyGo linux amd64
  planned:
    - TinyGo linux arm64
    - TinyGo wasip1 when networking or C dependencies are absent
compatibility_labels:
  - subset
  - interoperable
  - experimental
  - unsupported
evidence: https://tinygo.org/docs/reference/lang-support/stdlib/
```
