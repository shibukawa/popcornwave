---
id: requirement:httpbinder-extensible-route-analysis
type: requirement
title: Extensible httpbind-go Route Analysis
---
httpbind-go route analysis must recognize compatible routers and middleware through versioned declarative adapters instead of hard-coded package identifiers or selector-name guesses.

```yaml
owner: system:tinybind
defaults:
  - net/http package-level Handle and HandleFunc
  - net/http.ServeMux Handle and HandleFunc
adapter_schema:
  identity:
    - canonical Go import path
    - optional receiver type
    - function or method name
  route_registration:
    - pattern argument index
    - handler argument index
    - pattern grammar identifier
  transparent_wrapper:
    - handler argument index
  semantic_wrapper:
    - handler argument index
    - supported static metadata fields
resolution:
  - use go/types package and method identity, not source import alias or local variable name
  - accept renamed imports automatically
  - require literal or deterministically constant-folded route patterns
  - keep handler-body analysis in the application package
configuration:
  library_api: parser and generator options accept adapters programmatically
  cli: generator accepts a versioned analysis configuration file
  popcorn_web: api:cli-generate supplies the system:tinygodriver httpmux adapter
diagnostics:
  - report unsupported registration calls with source position and suggested adapter fields
  - reject invalid argument indexes, unknown grammars, and duplicate adapter identities
  - report routes whose handler body or Bind and Write calls cannot be resolved
security:
  - adapters are data and never execute user plugins or target code
  - package matching uses canonical import paths
  - configuration cannot load arbitrary host executables
compatibility:
  - zero configuration preserves net/http behavior
  - generated output remains deterministic for identical sources and adapters
  - adapter schema changes require an explicit version
candidate_followups:
  - explicit route annotation as a checked fallback when a registration API cannot be modeled
  - configurable aliases for Bind, Write, NewStream, and error constructor APIs
  - exported analysis result IR for framework tooling and diagnostics, promoted as data:route-table for rule:route-and-template-checks
acceptance:
  - aliased net/http imports resolve correctly
  - system:tinygodriver httpmux routes generate the same operations as net/http.ServeMux routes
  - unrelated methods named Handle or HandleFunc are ignored
  - malformed adapters fail before artifact generation
```
