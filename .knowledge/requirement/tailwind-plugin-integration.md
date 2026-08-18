---
id: requirement:tailwind-plugin-integration
type: requirement
title: Tailwind Plugin Integration
---
Tailwind plugins use Tailwind v4 CSS directives without a Popcorn Web plugin API or plugin-specific runtime integration.

```yaml
declaration:
  standard: '@plugin "<specifier>"'
  standalone_specifier: relative path to a local standalone-compatible ES module
  options: CSS block owned by the plugin
ownership:
  dependency: application
  configuration: configured CSS entry
  resolution_and_execution: Tailwind CLI
rules:
  - data:project-config contains no Tailwind plugin registry
  - api:cli-dev and api:cli-build pass the CSS entry to Tailwind unchanged
  - pin each local plugin module version and integrity before a reproducible build
  - builds never download missing plugin modules
  - bare npm package specifiers require a user-managed compatible toolchain
  - plugin JavaScript runs only in the host CSS build
references:
  - https://tailwindcss.com/docs/functions-and-directives#plugin-directive
```
