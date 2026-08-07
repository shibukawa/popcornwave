---
id: decision:separate-tinygo-dockerfile
type: decision
title: Separate TinyGo Dockerfile
---
A TinyGo project gets two container recipes as two files, Dockerfile and Dockerfile.tinygo, rather than one recipe parameterized by a build argument.

```yaml
status: accepted 2026-08-07, implemented
applies_to: requirement:container-image-scaffold
default: Dockerfile, the host Go recipe, is what docker build finds with no argument
alternatives_rejected:
  one_file_with_build_args:
    shape: ARG TOOLCHAIN with conditional stages selecting the base image and the compiler
    rejected_because:
      - the two recipes differ in base image, compiler invocation, and operational behaviour, so the conditionals would outnumber the shared lines
      - rule:tinygo-container-operations changes what the image can be asked to do, and a build argument hides that behind a flag rather than behind a file the reader opens
      - a reader comparing the two learns the cost of the toolchain answer, which is the thing they most need to know
  tinygo_only_for_a_tinygo_project:
    shape: write one Dockerfile whose content follows the toolchain answer
    rejected_because:
      - api:cli-build already builds a TinyGo project with host go, so the host recipe is the one that works today and the one every platform accepts
      - rule:tinygo-container-operations costs a TinyGo image its SIGTERM handling and its in-process migration apply, which is a deployment trade rather than a scaffold default
      - a project can switch back without regenerating anything
naming:
  file: Dockerfile.tinygo, not tinygo.Dockerfile
  reason: the name sorts beside Dockerfile and reads as a variant of it, matching config.{env}.toml in policy:config-file-resolution
  invocation: docker build -f Dockerfile.tinygo
non_default_reason: TinyGo produces the much smaller binary that decision:tinygo-042-baseline exists for, and the second file is where a project takes that trade deliberately
```
