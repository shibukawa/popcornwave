---
id: requirement:daisyui-integration
type: requirement
title: daisyUI Integration
---
daisyUI is an optional Tailwind plugin whose CSS-only component classes and themes remain compatible with Classic mode HTML.

```yaml
integration: requirement:tailwind-plugin-integration
standalone:
  dependency: pinned and checksummed local daisyui.mjs release bundle
  css: '@plugin "./plugins/daisyui.mjs"'
themes:
  optional_module: pinned and checksummed local daisyui-theme.mjs release bundle
  configuration: '@plugin "./plugins/daisyui-theme.mjs" { ... }'
usage:
  - selected in markup through data-theme
rules:
  - the application places official standalone modules and standard CSS declarations
  - Popcorn Web CLI and build code do not identify daisyUI by name
  - Popcorn Web does not wrap or reimplement daisyUI components
  - application HTML owns component structure and accessibility semantics
  - plugin version is independent from Popcorn Web releases
references:
  - https://daisyui.com/docs/install/standalone/
```
