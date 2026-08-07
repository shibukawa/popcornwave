---
id: decision:explicit-tinygo-compile-step
type: decision
title: Explicit TinyGo Compile Step
---
api:cli-build keeps building with host go, and a TinyGo build is api:cli-prepare followed by a tinygo build invocation the caller writes, rather than a compiler api:cli-build selects from data:project-config project.toolchain.

```yaml
status: accepted 2026-08-07, implemented
supersedes_the_open_gap: the api:cli-build note that a TinyGo build driven by that command would have to pass -scheduler=threads
shape_of_the_command:
  fact: api:cli-build is a preparation sequence followed by one compiler invocation as its last step
  consequence: splitting the command at that seam costs nothing, and a TinyGo build never runs the host link rather than running it and discarding the binary
  what_it_leaves_alone: api:cli-build behaviour, its flags, and every project that does not build containers
why_not_a_toolchain_driven_compiler:
  shape: api:cli-build resolves project.toolchain and invokes tinygo build with the flags the project implies
  attraction: Dockerfile.tinygo would be one command, and -scheduler=threads would come from configuration rather than from a Dockerfile line
  rejected_because:
    - api:cli-build would own a flag mapping for a compiler it does not otherwise know, and every new TinyGo flag becomes a framework decision
    - the compiler invocation is the one line a container build most often needs to change, for an output path, a target, or an optimization level, and burying it removes the reason to open the file
    - api:cli-dev builds with go run -tags=pwdev on the host whatever the toolchain answer was, so this would be the only command in the CLI that reads project.toolchain as a compiler selector
  not_a_reason: a second host link, which the api:cli-prepare split already removes from both shapes
residual_risk:
  what: -scheduler=threads lives in Dockerfile.tinygo, so switching requirement:database-engine-selection to a network engine later leaves the file silently wrong
  why_it_matters: rule:tinygo-runtime-compatibility measured the symptom as a query outliving its context deadline and returning a nil error, with nothing logged
  closed_by: the compiler, per the scheduler enforcement of rule:tinygo-runtime-compatibility; the engine package carries a guard file that fails the build unless the scheduler.threads tag is set
  why_that_and_not_a_check: the guard is keyed on the import graph, so it fires for every TinyGo build of a project that links the engine, however the build was invoked and whoever wrote the Dockerfile; a diagnostic would fire only when someone ran api:cli-doctor
  what_it_means_for_this_decision: the flag being in a file rather than in a command stops being a risk, because getting it wrong is now a compile error rather than a runtime behaviour
open:
  cross_compilation: go build reads GOOS and GOARCH from the environment and tinygo build does not, per the architecture note in rule:tinygo-container-operations; the explicit invocation is where a target is named, which is a second argument for this shape
```
