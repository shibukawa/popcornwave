---
id: requirement:package-schema-contribution
type: requirement
title: Package Schema Contribution
---
A concept:component-package keeps its own migration stream, applied before the application's, so a package upgrade carries its schema with it and no file is copied into the project.

```yaml
priority: should
problem:
  - a package cannot know which version numbers are free in the project's data:migration-source stream
  - a copy at install time is correct once and stale after every go get -u, and nothing forces the second command that would fix it
  - creating tables from startup or from an api:framework-extension Setup would write to a database at boot, which policy:migration-safety forbids
chosen: one stream per package, per decision:declared-package-installation
stream:
  source: a migration directory inside the module, named by data:component-package-manifest
  versioning: the package's own numbering, independent of the project's and of every other package's
  ledger: a version table per stream, named popcornweb_migrations_{stem} under the rule:framework-owned-tables prefix
  engine: the same system:goose provider api:migration-runner already constructs, with the table name selected per stream
ordering:
  rule: every package stream applies before the application stream, and a package's stream applies after the streams of the packages it imports
  derivation: the Go module import graph, which already expresses the only dependency direction that can exist
  why_it_is_sound: a package cannot reference an application table it has never seen, and an application may reference a package table; the import direction and the reference direction agree by construction
  constraint: a package migration may reference only its own tables and those of a package it imports; anything else is an ordering the graph does not describe
  determinism: the graph is a total order after tie-breaking on module path, so two runs apply the same sequence
readers:
  in_process: the embedded fs.FS registered through api:package-registration, used by the decision:migration-execution-split in-process path and by api:test-run
  cli: api:cli-migrate resolves each declared module's directory through the Go tool and reads the same files from the module cache, because a CLI has no access to another binary's embedded data
  invariant: both readers see identical bytes, because the embed and the module directory are the same files; a package whose embed pattern misses a migration is a package whose two readers disagree, and its own release check must catch it
  tinygo: the delegated path stages each stream to its own temporary directory, per decision:migration-execution-split
reporting:
  before_applying: api:cli-migrate lists pending package migrations, their stream, and their statements, which is what replaces the review a copy into the project used to get
  status: api:migration-runner Status reports every stream, so one command answers what the database actually carries
  doctor: api:cli-doctor reports a declared package whose stream has never been applied
upgrade:
  action: go get -u, then api:cli-migrate; the new versions are pending in that package's stream and apply in order
  no_second_install: nothing is copied, so nothing can be forgotten
downgrade:
  behavior: a package version with fewer migrations leaves applied versions with no source, which api:migration-runner already classifies as a recorded version ahead of available sources
  rule: reported, never auto-reverted; a package downgrade is not a schema downgrade
naming:
  tables: a package's tables carry its own prefix, declared in the manifest stem, so two packages cannot claim one name silently
  collision: two declared packages publishing the same stem is a startup and a doctor error
acceptance:
  - declaring a package and running api:cli-migrate creates its tables with no file written into the project
  - upgrading the package and running api:cli-migrate applies only the versions added since
  - the application's own migrations still apply after every package's, whatever their numbers
  - a package that imports another has its stream applied after that one
  - api:cli-migrate prints every pending package statement before applying it
  - api:test-run creates package tables from the embedded stream with no CLI involved
  - a package whose embedded stream and module directory disagree fails its own release check
  - removing a declaration leaves the applied tables in place and reports them as unowned
```
