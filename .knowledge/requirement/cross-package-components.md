---
id: requirement:cross-package-components
type: requirement
title: A Component Must Be Callable From Another Module
---
An external component declaration must be able to name a target in another Go module, resolved from what that module published rather than from its template sources, because a consumer never has those sources under generation.

```yaml
owner: system:tinybind
status: not raised upstream as of 2026-08-02; the upstream catalog carries the same gap as an open question on its template file scope requirement, and its library component seam decision states the library case is open
priority: should
blocks: the component-library shape of concept:component-package, per requirement:component-package-distribution
what_exists_upstream:
  cross_file: an external component declaration resolves a component exported from another template file, restating its parameters, slots, and output type, bound at generation time
  modular_generation: one generator invocation analyzes one Go package, and an imported generated package is reused without regeneration, which is exactly the model decision:committed-package-artifacts needs
  open_question: whether an external declaration names a path, a module, or a configured generation unit, deferred rather than answered
  library_case: the upstream component seam decision records that a registered or cross-package component still has no way in, and that its asset seam has to serve both
what_is_missing:
  target_notation: an external declaration has no spelling for a component that is not in this generation unit
  resolution_input: cross-file resolution reads the other template file; a consumer has the dependency's generated Go and not its .pw.html, and reading the module cache's templates would be the dependency rescan the modular model forbids
  signature_evidence: the generated package exposes component parameter types in-process, and nothing persists them in a form a separate generator run can verify a declaration against
why_the_framework_cannot_close_it:
  - the declaration is template syntax, and the framework supplies no parser
  - the check is a generation-time type check between two independently generated units, which is the generator's own contract with itself
  - the framework can pass options and register call patterns, and none of those adds a resolvable name to a template
proposed_shape:
  notation: an external declaration naming a Go import path plus the exported component name, since the import path is the identity the consumer already resolved through go.mod
  resolution: read the dependency's generated package, not its templates, so the modular rule holds and the module cache stays read-only
  evidence: the generated package carries an exported, machine-readable signature for each exported component, emitted beside the renderer, so a declaration is verified against a published artifact rather than against source
  verification: unchanged from the cross-file case; parameter names, types, slots, and output type must match, and a mismatch names both positions
  version_skew: the signature artifact is the compatibility surface, so a package changing a component signature breaks its consumers at generation with a diagnostic rather than at link with a Go type error
acceptance:
  - a component exported by an imported module is callable from a consumer template with full parameter type checking
  - the consumer's generation reads no .pw.html outside its own project
  - a slot on a cross-module component is filled exactly as a same-file one is
  - a signature change in the dependency fails the consumer's generation, naming the dependency version and both positions
  - a project using no cross-module component regenerates byte-identical output
related:
  - requirement:package-component-assets, which is the same boundary seen from the asset side
  - requirement:generated-code-version-tolerance
```
