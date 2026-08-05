---
id: api:package-registration
type: api
title: Package Registration
---
A linked concept:component-package registers its identity, its embedded assets, and its migration stream from init, carrying only what the framework must read without the application naming it.

```yaml
surface:
  - pw.RegisterPackage(pw.Package) called from an imported package init
  - Package fields are Module, Version, Assets, and Migrations
  - Module is the Go module path, matching the data:component-package-manifest module key
  - Version is the module version the package was built at, supplied as a constant
  - Assets is an fs.FS of embedded browser files, or nil
  - Migrations is an fs.FS of the requirement:package-schema-contribution stream, or nil
routes:
  not_here: a package serving routes exposes an ordinary Register function and concept:application-entry-point calls it, exactly as it calls the generated api:page-registry of its own concept:page-tree
  why_not_automatic: an installed route contributor would be the framework route registration API api:framework-extension states does not exist, and mounting is the one contribution an application has an opinion about
  cost: this is the single step decision:declared-package-installation does not remove; a route-serving package is one declaration plus one call
  consequence: such a package is linked by the application's own import, so its generated blank import is redundant rather than load-bearing
what_is_not_here:
  middleware_and_startup: api:framework-extension, unchanged; a package contributing middleware uses the existing call
  configuration: the generated pw.RegisterConfig binding, which already registers from init and needs no package awareness
  linking: the generated bootstrap blank import, which api:cli-generate emits from the declaration
  rationale: a second mechanism for a job an existing one does would double the surface and halve the review of both
rules:
  - registration completes before ParseConfig, matching api:framework-extension
  - reject a duplicate module path, and reject a migration stem two packages share
  - a package registering only Assets installs no middleware and no route
  - registration installs nothing that answers a request; every request path a package serves is mounted by the application
  - Assets and Migrations are read at mount and at migrate time, never per request and never from the filesystem
  - a linked package the application never declared is a api:cli-doctor finding, because a transitive dependency contributing assets or a schema is a surprise the declaration exists to prevent
  - the framework never calls into a package beyond these fields; a package contributes and the framework composes
  - no runtime lookup by name, no reflection, and no init-order dependency between two packages
identity:
  used_for: the requirement:package-asset-delivery URL derivation, the requirement:package-schema-contribution stream ledger, and the api:cli-doctor report of what the binary actually linked
  exposed: pw.Packages returns the registered set, so a running application answers what it carries
consumers:
  - requirement:package-asset-delivery
  - requirement:package-schema-contribution
  - api:cli-doctor
  - flow:component-package-consumption
```
