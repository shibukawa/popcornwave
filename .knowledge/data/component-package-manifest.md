---
id: data:component-package-manifest
type: data
title: Component Package Manifest
---
The package section of a concept:component-package's data:project-config declares what the module publishes, and the consuming project's packages array declares which modules it uses.

```yaml
files:
  published: popcornweb.toml in the package module root, the same file api:cli-generate already reads for the package's own generation
  consuming: popcornweb.toml in the application, whose packages array names the modules, per decision:declared-package-installation
reachability: shipped inside the module, so the consumer's CLI reads the published half from the module cache with no network access and no build
published_schema:
  selector: project.kind = "package", which is also what makes project.main optional
  package:
    module: the Go module path, repeated from go.mod so a copied file is detectable
    summary: one line, shown when api:cli-package offers or reports the package
    import: the package path an application links, when it differs from the module root
    requires:
      capabilities: the requirement:incremental-project-capabilities members the package needs, such as database
      engines: the requirement:database-engine-selection engines the package supports, empty meaning it touches no SQL
    generated_with:
      pw: the Popcorn Web version that generated the committed artifacts
      tinybind: the system:tinybind version behind it
      consumer: policy:package-compatibility decides what a mismatch means
    config:
      section: the runtime configuration section the package registers, so api:cli-doctor can report it and api:cli-generate can scaffold it on request
      never: the section is not written into an environment file at install; the package's registered defaults already apply
    migrations:
      dir: the directory inside the module holding the requirement:package-schema-contribution stream
      stem: the identity carried by the stream's version table and by the package's table names
      engines: the engines the stream is written for, which must not be narrower than requires.engines
    routes:
      register: the exported symbol concept:application-entry-point calls to mount the package, empty for a package that serves no route
      purpose: api:cli-package prints the call and api:cli-doctor reports a declared package whose symbol is never called; the framework never calls it, per api:package-registration
    assets:
      declared: whether the package registers embedded assets through api:package-registration
      note: no path or URL appears here, because requirement:package-asset-delivery derives both from content
    components:
      exported: the .pw.html components a consumer template may call
      stage: empty until requirement:cross-package-components ships, and an entry before then is a load error rather than a promise
consuming_schema:
  packages:
    form: an array of tables, one per module the application uses
    module: the module path, which must be in go.mod
    minimum: the module path alone is a complete entry
rules:
  - the published half is read, never written, by the consumer; api:cli-package writes it in the package repository
  - an unknown key is an error, matching data:project-config
  - a package section in a project.kind = "application" file is an error, and project.main in a package file is the same error from the other side
  - the published half declares contributions and never their placement; the framework decides the asset URL and the migration order
  - a declared module with no package section is an error, and an undeclared module with one is an ordinary Go dependency no command treats as a capability
  - nothing is discovered; a contribution absent from the published half is absent, even when the Go code performs it
consumers:
  api:cli-generate: emits the blank import for every declared package, which is what links it
  api:cli-migrate: locates each declared package's migration stream through the Go tool
  api:cli-doctor: reports version mismatches, unapplied streams, declared modules missing from go.mod, and linked packages the project never declared
discovery:
  how: the packages array, resolved against the module graph
  why_not_the_module_graph_alone: go.mod says a module is available, including transitively; the array says the application intends to use it, and the bootstrap generator needs the second fact
  why_not_a_separate_state_file: a project-local record of installed packages would disagree with popcornweb.toml, which is the reasoning requirement:incremental-project-capabilities used to reject a capability manifest
```
