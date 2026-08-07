---
id: requirement:dockerless-image-builders
type: requirement
title: Dockerless Image Builders
---
ko and Cloud Native Buildpacks get one note on the container images page rather than a page of their own, because everything a Popcorn Wave project needs to know about them is that they own the compile step and therefore run after api:cli-prepare.

```yaml
support_tier:
  documented: a single note-level admonition naming both builders and the one rule that makes them work
  not_scaffolded: api:cli-init writes no .ko.yaml and no project.toml; a project that wants one has a platform that told it to
  not_verified_per_release: the requirement:container-image-scaffold acceptance criteria exercise the Dockerfile path only
  why_it_stays_small: the interesting content would be each builder's own configuration surface, which is that builder's documentation and goes stale here
the_note:
  says:
    - both builders replace the Dockerfile, not the rule:container-build-inputs host phase, so pw prepare runs first in the working tree and the builder is invoked over what it left behind
    - it works because both read the working directory rather than the git index, so the generated Go and dist/public that .gitignore excludes are present and are used
    - a CI job that checks out and invokes the builder directly gets the rule:container-build-inputs failure, reported as missing symbols rather than as a missing step
    - neither invokes any compiler but go, so rule:tinygo-container-operations has no dockerless path
    - a Docker HEALTHCHECK is a Dockerfile instruction, so neither produces one; a platform probe against the configured health path replaces it, and requirement:healthcheck-subcommand keeps its exec-form value wherever the binary is still invoked
  does_not_say: builder versions, builder names, image configuration keys, or a worked command sequence
recommendation: the scaffolded Dockerfile, because it is the only path where the host phase, the toolchain pin, the probe, and the configuration file are all in one reviewable file; a builder is the answer when the platform already runs one for every other service and the project is host Go
non_goals:
  - a builder-specific scaffold, configuration file, or api:cli-doctor check
  - a Popcorn Wave buildpack that would run the host phase, which is a maintained artifact per platform and a distribution channel requirement:cli-distribution does not have
```
