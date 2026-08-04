---
id: vision:popcorn-wave
type: vision
title: Popcorn Wave Vision
---
Popcorn Wave is a compact Go and TinyGo web application framework that preserves net/http familiarity while removing routine binding, rendering, SQL, serialization, and lifecycle boilerplate.

```yaml
goals:
  - a useful application starts with pw init, devbox shell, and pw dev
  - handwritten application code normally imports only github.com/shibukawa/popcornwave/pw
  - generated binding, HTML, SQL, and serialization remain reviewable beside their sources
  - standard net/http handlers and custom server lifecycles remain available
  - the same application structure works with Go and TinyGo
principles:
  - concept:public-package-boundaries
  - decision:host-tools-target-runtime
  - decision:stdlib-servemux
  - policy:generated-artifacts
non_goals:
  - custom router DSL
  - runtime dependency injection container
  - ORM
  - browser frontend framework
  - WASI HTTP support in the first release
primary_actor: actor:application-developer
acceptance: requirement:mvp-acceptance
authoritative_experience: requirement:application-user-experience
runtime_expansion: vision:contrib
third_party_expansion: requirement:component-package-distribution, which opens the same reach to modules published outside this repository
application_styles: vision:web-application-styles
editor_experience: vision:editor-support
```
