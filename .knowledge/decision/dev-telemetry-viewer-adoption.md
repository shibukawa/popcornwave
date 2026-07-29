---
id: decision:dev-telemetry-viewer-adoption
type: decision
title: Development Telemetry Viewer Adoption
---
system:localotelviewer is linked for its receiver and copied for its UI sources, rather than reimplemented, vendored whole, or spawned as a foreign process.

```yaml
status: accepted
context:
  - requirement:contrib-otel defines trace and log export but names no local destination, so nothing in the developer loop reads what it emits
  - api:cli-dev already owns service startup, process lifetime, and credential injection for requirement:contrib-devidp
  - receiver, bounded store, and UI already exist upstream and are maintained by the same author
license_analysis:
  pw_license: Apache-2.0, declared by requirement:cli-distribution and data:release-artifact
  module_shape: cmd/pw and internal/pwcli share one Go module with the pw, pwruntime, and contrib packages that link into application binaries
  consequence: AGPL code anywhere in that module would reach user applications, so AGPL scoped to api:cli-dev alone is not expressible without splitting the module
  resolution: upstream is dual-licensed AGPL-3.0-only OR Apache-2.0, and pw takes the Apache-2.0 option
  effect: requirement:cli-distribution, decision:homebrew-tap-channel, and decision:nix-flake-packaging are unchanged
split:
  go: linked from the upstream viewer package, so the receiver, store, and snapshot API have one implementation and one maintainer
  ui:
    mechanism: upstream React component sources are built into a bundle committed under internal/otelui and embedded
    why_copied: the public Go package ships no assets on purpose, and the component is not published to npm
    why_built_here: a committed bundle keeps the pure-Go release pipeline of data:release-artifact free of a Node toolchain
    cost: the bundle is refreshed by hand when the upstream component changes
    value: pw owns the mount point, so a later admin console can host the same component beside its own surfaces
alternatives_rejected:
  reimplement_subset:
    precedent: decision:devidp-scope-reduction did exactly this for system:oidcld
    why_not_here: the value of this dependency is the UI, so a reduction reproduces the expensive part and saves nothing
  vendor_the_receiver:
    why_not: the Go half is exported and versioned, so copying it would fork maintained code for no gain
  spawn_external_binary:
    shape: run the viewer as a child process like a Devbox service
    why_not: it adds an install prerequisite to a first-run developer loop that api:cli-dev otherwise satisfies from one binary
  agpl_the_cli:
    shape: split cmd/pw and internal/pwcli into an AGPL module
    why_not: it reopens every distribution channel for a development-only view
attribution: internal/otelui carries the upstream copyright notice and the SPDX expression
```
