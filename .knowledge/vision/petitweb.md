---
id: vision:petitweb
type: vision
title: Petitweb Vision
---
Petitweb is a small TinyGo-oriented web application framework that preserves net/http familiarity while automating project setup and generated HTTP mapping.

```yaml
goals:
  - one-command project bootstrap
  - typed request and response handling through system:httpbinder
  - consistent binding and validation errors
  - host-side generation with a reflection-free target runtime
  - native TinyGo server builds in the first release
principles:
  - decision:thin-httpbinder-integration
  - decision:host-tools-target-runtime
  - decision:stdlib-servemux
non_goals:
  - custom router DSL
  - runtime dependency injection container
  - ORM or database abstraction
  - browser frontend framework
  - WASI HTTP support in the first release
primary_actor: actor:application-developer
acceptance: requirement:mvp-acceptance
runtime_expansion: vision:contrib
```
