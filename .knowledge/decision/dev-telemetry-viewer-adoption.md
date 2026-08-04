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
    mechanism: linked from the upstream viewer/webui subpackage, which embeds the committed build
    linkage: importing the viewer alone still links no assets, so this import is the explicit choice to take them
    pipeline: the assets arrive as Go source, so data:release-artifact stays a pure-Go cross-compile with no Node toolchain
    value: pw owns the mount point, which is what let requirement:dev-console host the same component beside its own panes without renegotiating with the dependency
  superseded_copy:
    was: a bundle built from upstream component sources and committed under internal/otelui, because the public package shipped no assets
    cost_paid: the copy was refreshed by hand on every upstream change, and it turned out to differ from the upstream build only in two lines of index.html
    resolved_by: upstream v1.0.2, which exports the assets, makes their references relative, resolves the snapshot API against the served document, and separates the displayed OTLP endpoint from the API base
    lesson: the divergence was worth reporting rather than absorbing; every part of the copy existed to work around something the dependency could fix once for every embedder
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
