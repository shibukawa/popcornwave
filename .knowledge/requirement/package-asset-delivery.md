---
id: requirement:package-asset-delivery
type: requirement
title: Package Asset Delivery
---
A concept:component-package's browser files are served from its own embedded bytes under a reserved content-addressed path, so a component's script reaches a page the package cannot see.

```yaml
priority: should
problem:
  - a component is a template plus a script, and the script has no home; the template compiles into the package and the script has nowhere to be served from
  - requirement:public-asset-delivery serves the project's public tree, which a dependency cannot write into
  - requirement:framework-script-assets solves the identical problem for framework-owned scripts, and its answer only covers files this repository ships
location:
  path: "/_pw/pkg/<digest>/<name>"
  reserved: under the same fixed prefix requirement:framework-script-assets already claims, ahead of application routing
  rationale:
    - a package asset is no more an application asset than a framework one is, so the application mount has no say over it
    - one reserved prefix answers 404 for an unknown path under it, so a package asset cannot fall through into an application route
    - it stays available when requirement:public-asset-delivery is disabled, for the same reason the framework scripts do
digest:
  value: a content digest over the file bytes
  per_file: unlike the framework's shared revision segment, because a package set is contributed by several independent publishers and has no single version to hash
  consequences:
    dedup: two packages embedding identical bytes produce one URL and one served file
    immutability: a changed file is a changed URL, so the cache header stays honest
    upgrade: a package upgrade moves only the URLs whose bytes moved
  rejected: a per-package version segment, which would refetch every file of a package for a change to one and would collide when two packages ship the same name
source: the Assets fs.FS of api:package-registration, walked once at startup
delivery:
  headers: public, max-age one year, immutable, matching requirement:framework-script-assets
  compression: precompressed variants are not embedded; a package asset is served as stored, and policy:public-asset-precompression stays a project build concern
  no_filesystem: nothing is written into the project tree and nothing is read from disk at request time, which is what keeps a TinyGo target working
referencing:
  today: the application names the URL, obtained from the framework by asset name and package identity
  wanted: the component declares the asset and the framework supplies the URL function, which is requirement:package-component-assets upstream and is what removes the application from the loop
  interim: a package documents its asset name and the application writes one tag in its document shell, which works and does not scale past a few packages
security:
  - a package asset is static content chosen at build time, so a page needs no inline script and policy:security-response-headers can keep script-src to self
  - a digest path is not a capability; the same bytes are served to anyone, and nothing request-scoped enters an asset
rules:
  - an asset is always a reference to a served file, never inlined
  - the mount is registered once at startup and rejects nothing, because a digest collision across distinct bytes is the hash's problem and not a configuration one
  - a package registering no Assets adds no route
acceptance:
  - a page using a component from an imported package loads that component's script with no file copied into the project
  - two packages embedding the same file are served from one URL
  - changing one byte of one asset changes exactly that URL
  - disabling requirement:public-asset-delivery leaves package assets reachable
  - a TinyGo build serves every package asset with no filesystem access
non_goals:
  - bundling, minification, or any JavaScript toolchain
  - a package choosing its own URL, mount path, or cache policy
  - copying package assets into the project public tree
```
