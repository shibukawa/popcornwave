---
id: policy:operational-endpoints
type: policy
title: Operational Endpoints
---
The primary HTTP listener may expose minimal health, readiness, and generated OpenAPI endpoints configured by data:server-runtime-config.

```yaml
health:
  methods: GET and HEAD
  success: process is serving and not shutting down
  dependencies: none
readiness:
  methods: GET and HEAD
  success: startup completed and every enabled critical middleware resource reports ready
  failure: HTTP 503
  checks:
    - bounded and context-aware
    - include api:rdb-middleware and selected session backend availability
openapi:
  methods: GET and HEAD
  response: generated OpenAPI document
  requirement: build-time artifact enabled in data:project-config
access:
  - health and readiness bypass session and authentication and reveal only status
  - OpenAPI follows policy:authenticated-path-protection like an application route
rules:
  - paths are unique absolute paths on the primary listener
  - return no DSN, backend name, stack, configuration, or dependency detail
  - disabled endpoints register no route
non_goals:
  - static file serving; applications compose embed, fs.FS, and net/http handlers
  - metrics endpoint in the first release
  - hosted API documentation UI
```
