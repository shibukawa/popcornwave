---
id: decision:committed-package-artifacts
type: decision
title: Committed Package Artifacts
---
A concept:component-package commits its generated Go, inverting the policy:generated-artifacts rule that an application excludes it, because nothing in the consumer can recreate it.

```yaml
status: accepted
inverted_rule:
  application: generated Go is excluded from version control and recreated by api:cli-generate before every build
  package: generated Go is committed, because the consumer's build is go build and its generator never reads a dependency
  what_stays: every other policy:generated-artifacts rule, including the header, the name pattern, the ban on manual edits, and atomic replacement
why_the_consumer_cannot_generate:
  - the module cache is read-only, and writing into it would make one project's build change another's dependency
  - decision:explicit-generation-sources lists project-relative directories, so a dependency is outside every purpose by construction
  - a generated artifact depends on the generator version, and a consumer regenerating a dependency would silently republish it under a version its author never tested
  - the system:tinybind modular generation model states the same rule from the other side: one invocation analyzes one package, and an imported generated package is reused rather than regenerated
consequences:
  review: a package's diff carries generated Go, which is noise in review and is the price of being installable
  authority: the source is still authoritative; the committed artifact is a build output that happens to be tracked
  staleness: a package whose commit is stale fails to compile in the consumer, which is the loudest available failure and is why no repair path is offered
guards:
  package_ci: api:cli-generate --check runs over the package and fails when a commit is stale, scaffolded by requirement:package-project-scaffold rather than left for the author to remember
  publish: the same check is the release gate, so a tag never carries an artifact its source does not produce
  consumer: nothing; the consumer trusts the artifact exactly as it trusts any other compiled dependency
scaffolding:
  api:cli-package: the package .gitignore carries no **/*_pw_gen.go rule, which is the one line that differs from the api:cli-init application scaffold
  editor: the .vscode explorer hide rule is kept, because hiding a file is not excluding it
public_assets:
  extracted: a component style or script the generator extracts is committed too, on the same reasoning
  precompressed: a public/**/*.zstd sidecar stays excluded, because requirement:package-asset-delivery serves package assets from embedded bytes and the sidecar is a project build product
rejected_alternatives:
  vendor_the_sources: the consumer copies a dependency's .pw.html and generates it locally, which reintroduces the version skew the module system exists to remove
  generate_on_install: a post-download hook, which Go has no notion of and which would make go build depend on a tool the consumer may not have
  ship_only_go: publish handwritten Go and no templates, which is what a package can already do today and is the absence of this feature rather than an answer to it
```
