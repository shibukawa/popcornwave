---
id: requirement:dev-asset-inspector
type: requirement
title: Development Asset Inspector
---
A requirement:dev-console pane reports the public tree as the developer loop serves it and as api:cli-build would serve it, because decision:development-public-assets makes those two answers differ and nothing today shows the difference.

```yaml
audience: actor:application-developer
pane_of: requirement:dev-console
source: static analysis of the project tree, per policy:dev-console-boundary
problem:
  - api:cli-dev reads files locally, negotiates no encoding, and creates no sidecar
  - so the first time a developer sees what requirement:public-asset-delivery actually serves is after a release build
shows:
  file_tree:
    entries: every file under project-root public, excluding the generated sidecars and the empty-tree sentinel of flow:public-asset-build
    per_entry:
      - the URL it answers, resolved through server.public.mount
      - size
      - whether policy:public-asset-precompression finds it eligible
      - the sidecar flow:public-asset-build would write, and the ratio it would reach
      - whether a sidecar exists now and whether it is stale against its source
  mode:
    current: the decision:development-public-assets reading, stated plainly so nothing on this page is mistaken for production behavior
    production: the requirement:public-asset-delivery reading the same configuration would produce
  generated:
    tailwind: the requirement:tailwind-css-integration output path, whether it exists, and whether it is older than its input
    framework_scripts: the requirement:framework-script-assets set and the revision segment currently serving them
  configuration: the data:server-runtime-config public values that produced the mount, with the source layer each came from
limits:
  - a mount collision is reported by api:cli-doctor rather than here, because rule:project-integrity-checks already owns it
  - a value the pane could not determine is named as undetermined rather than shown as its default
actions:
  none: this pane runs no build
  reason: policy:dev-console-boundary admits only actions that already exist as a subcommand, and precompression belongs to api:cli-build
non_goals:
  - editing, uploading, or deleting a file
  - an asset pipeline, a bundler, or a fingerprinting scheme
  - reporting request-level asset traffic, which requirement:dev-telemetry-viewer already carries
acceptance:
  - a project with no public directory reports an empty tree rather than an error
  - an eligible file with no sidecar is shown as one production would compress
  - an ineligible file names the policy:public-asset-precompression reason it was skipped
  - a stale Tailwind output is reported as stale, with the input that is newer
  - the pane is correct while the application process is down, because it reads the tree and not the server
```
