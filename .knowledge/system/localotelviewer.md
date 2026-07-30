---
id: system:localotelviewer
type: system
title: LocalOtelViewer
---
LocalOtelViewer is a local OpenTelemetry receiver whose public package requirement:dev-telemetry-viewer links for the Go half and whose React sources it builds for the UI half.

```yaml
repository: https://github.com/shibukawa/localotelviewer
version: v1.0.1
license:
  spdx: AGPL-3.0-only OR Apache-2.0
  used_by_pw: Apache-2.0, which matches this repository
  rationale: decision:dev-telemetry-viewer-adoption
role: library dependency of system:pw-cli, unlike system:oidcld which stays prior art
go_api:
  package: github.com/shibukawa/localotelviewer/viewer
  surface:
    - NewHandler(max, options) returns a listener-independent http.Handler
    - New(address, max, options) owns a listener; "127.0.0.1:0" selects a free port
    - WithWebHandler(handler) mounts a caller-supplied UI
    - Server.URL, Server.Shutdown, Server.MonitorProcess
  routes: /v1/traces, /v1/logs, /v1/metrics, and /api/snapshot
  encodings: OTLP/HTTP protobuf and JSON
  storage: bounded in-memory only, discarded at exit
  assets: none; the public package deliberately ships no UI so an embedding tool brings its own
ui_source:
  component: OtelViewer React component with apiEndpoint, theme, onThemeChange, initialView, and pollIntervalMs
  distribution: repository sources only, not published to npm
  consumer: requirement:dev-telemetry-viewer builds them into a committed bundle
unused_by_pw:
  - per-language process wrappers, because api:cli-dev starts one Go process it already owns
  - instrumentation download, because requirement:contrib-otel instruments in-process
  - the CLI binary and its own embedded UI build
```
