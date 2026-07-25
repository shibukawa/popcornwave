---
id: flow:tailwind-css-build
type: flow
title: Tailwind CSS Build Flow
---
Popcorn Wave supervises the configured Tailwind host process without importing Tailwind into the application runtime.

```yaml
development:
  trigger: api:cli-dev
  steps:
    - validate decision:tailwind-host-toolchain
    - let Tailwind resolve requirement:tailwind-plugin-integration declarations from the CSS entry
    - start Tailwind with input, output, and watch mode
    - report prefixed diagnostics beside Go generation and build diagnostics
    - let decision:development-public-assets serve output under public without a Go rebuild
    - rebuild only when a configured output outside public is an application build input
    - terminate the child process during restart or shutdown
production:
  trigger: api:cli-build
  steps:
    - validate locked tool availability
    - fail when a declared local plugin module is missing
    - write minified CSS to a temporary sibling path
    - replace configured output atomically after success
    - run the Go build only with current CSS
failure:
  - preserve the previous successful CSS file
  - return nonzero for production builds
  - keep development supervision alive and wait for source correction
```
