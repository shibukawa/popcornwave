---
id: requirement:mvp-acceptance
type: requirement
title: MVP Acceptance Criteria
---
The Popcorn Web MVP is complete when the documented three-command startup yields a generated, testable, running Go application that preserves its TinyGo-compatible runtime design.

```yaml
criteria:
  - api:cli-init creates concept:project-layout in an empty temporary directory
  - devbox shell provides the declared development tools and default Valkey service
  - generated starter passes "go test ./..."
  - api:cli-generate is deterministic and its check mode detects drift
  - starter demonstrates api:request-binding, api:html-response, and flow:sql-generation
  - malformed input produces policy:validation-errors problem details
  - generated OpenAPI assembles every imported package fragment deterministically
  - api:cli-dev watches, regenerates, rebuilds, and restarts the starter
  - api:cli-build generates and compiles data:project-config project.main
  - api:cli-init creates stable public.go and flow:public-asset-build embeds originals with eligible .zstd sidecars
quality:
  - CLI command tests cover success, collision, and failure paths
  - generated project smoke test runs outside the Popcorn Web repository
  - no reflection-based field mapping is introduced
optional_css_profile:
  - api:cli-init --tailwind creates a Devbox-pinned standalone Tailwind toolchain without Node project files
  - a local ES module plugin fixture builds through the standard @plugin declaration
  - requirement:daisyui-integration passes the same plugin path without CLI-specific handling
  - api:cli-dev supervises CSS watch and Go rebuild without leaking a child process
  - api:cli-build produces minified CSS before embedding and Go compilation
  - target binary has no Node.js, Tailwind, or daisyUI runtime dependency
public_assets:
  - api:public-asset-middleware serves embedded assets at the configured mount
  - api:cli-dev serves project public files directly without compression, embedding fallback, or Go rebuild
  - server.public.read_local tests local override, embedded fallback, and layer-consistent sidecars
  - production Accept-Encoding tests zstd, wildcard, q=0, identity fallback, Vary, HEAD, and 406
  - traversal, symbolic link, dot-path, directory-listing, and direct .zstd requests fail closed
```
