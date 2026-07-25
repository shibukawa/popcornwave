---
id: requirement:tailwind-css-integration
type: requirement
title: Tailwind CSS Integration
---
Optional Tailwind CSS v4 tooling scans Popcorn Wave sources and emits one reproducible static stylesheet for application-owned delivery.

```yaml
inputs:
  - configured CSS entry containing @import tailwindcss
  - .pw.html templates
  - Go sources containing complete literal class names when explicitly sourced
  - CSS-owned @source, theme, utility, variant, and requirement:tailwind-plugin-integration directives
output: configured static CSS path
discovery:
  - treat .pw.html as plain text source
  - require complete class-name literals
  - reject or warn on template expressions that construct class-name fragments
  - use @source inline for intentional safelists
runtime:
  - default output enters requirement:public-asset-delivery
  - a custom output remains application-owned
rules:
  - flow:tailwind-css-build runs only when configured
  - CSS failure prevents a production build from using stale output
  - Popcorn Wave does not parse, translate, or own Tailwind CSS configuration
  - generated CSS is reproducible but is not policy:generated-artifacts generated Go
references:
  - https://tailwindcss.com/docs/installation/tailwind-cli
  - https://tailwindcss.com/docs/detecting-classes-in-source-files
```
