---
id: requirement:component-asset-extraction
type: requirement
title: Component Asset Extraction
---
A component's style and script blocks become content-hashed files under the project public tree instead of inline markup in the merged head, so a page links them, a browser caches them, and a policy can forbid inline style and script outright.

```yaml
source: gap found while surveying system:tinybind v0.3.0
not_this_delivery:
  scope: this is about assets an application component declares, and is unrelated to how the framework's own runtime reaches the page
  that_is: decision:runtime-tag-injection, which contributes the framework script at the render call so no application file can drop it
  standing: a real gap worth closing on its own schedule, deliberately not sequenced with the update capabilities
today: api:cli-generate configures neither asset option, so every component style and script reaches the response inline; the extraction exists upstream and this project does not ask for it
motivation:
  caching: inline bytes are re-sent with every document and cached with none of them
  policy: policy:security-response-headers cannot narrow style-src or script-src while a component can emit an inline block
  authored_islands: the authored-islands tier of concept:interaction-cost-ladder is the one that needs a component to ship JavaScript, and today that JavaScript has nowhere to be but the head
generation:
  options: the public directory and the URL base of system:tinybind, set by api:cli-generate rather than by each project
  paths: aligned with requirement:public-asset-delivery, so extracted files land under the project public tree the embed and the middleware already cover
  naming: the generation unit name, the asset kind, and a content hash, so the URL is immutably cacheable and an unchanged project regenerates identical names
  bundling: styles of one template file bundle into one stylesheet, while each component script stays its own file so author attributes survive on its tag
  passthrough: a script or link already naming an external URL contributes its tag unchanged and produces no file
  determinism: identical sources produce identical names and bytes, which is what lets rule:project-integrity-checks compare a regeneration
  generated_artifact: an extracted file is a generated artifact under policy:generated-artifacts, so it is reviewable beside its source and never hand edited
delivery:
  serving: api:public-asset-middleware, unchanged; an extracted file is an ordinary public asset
  precompression: flow:public-asset-build compresses it like any other, per policy:public-asset-precompression
  development: decision:development-public-assets serves it from disk, so a regeneration is visible without a rebuild
  disabled_endpoint: a project disabling requirement:public-asset-delivery while a component declares an asset is a startup failure, since the reference would resolve to nothing
  distinct_from: requirement:framework-script-assets, which serves framework-owned scripts from a reserved path and is never an extracted artifact
fragment_path:
  rule: decision:fragment-head-rejection is unchanged; a fragment response still refuses a component declaring head contributions, and an asset reference is one
  reason: the reference lives in the head, so a swapped region cannot carry it, and dropping it silently would swap in an unstyled region
update_path:
  problem: a navigation delta or a redraw can insert a component whose script the first render never linked
  handled_by: the head operations of requirement:navigation-delta-rendering, which install a first-appearing component's tags before its markup is applied
  closed_v0_3_3: the redraw registry reports the head and assets every published component needs, so the document shell carries them once at startup, and a redraw whose component contributes head also announces it on a response header
scope_today:
  covered: a component declared in a template file of the generation unit being compiled
  not_covered: a component a library registers or declares in another package, which owns no route and no shell and therefore cannot reference its own file
  shipped_v0_3_3: the required-asset set is on the bound value and readable before rendering, folded across the call graph and through slots, identified by the same content-hashed name extraction already computes
  still_missing: the embedded byte table and the caller-supplied URL function, which is what a TinyGo target with no filesystem needs
acceptance:
  - a component shipping a style and a script produces two files and two head tags, and no inline block reaches any response
  - an unchanged project regenerates identical file names, so caches stay valid
  - a component referencing an external CDN script emits a tag and no file
  - a page linking an extracted asset is served correctly with the public endpoint enabled, and fails at startup with it disabled
  - policy:security-response-headers can be tightened to forbid inline style and script without breaking a scaffolded project
  - a project declaring no component asset regenerates byte-identical output
```
