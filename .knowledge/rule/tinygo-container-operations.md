---
id: rule:tinygo-container-operations
type: rule
title: TinyGo Container Operations
---
A TinyGo image is smaller and behaves differently under an orchestrator, because two rule:tinygo-runtime-compatibility platform gaps become operational facts the moment the process is one container among many.

```yaml
applies_to: an image built from Dockerfile.tinygo, per decision:separate-tinygo-dockerfile
no_graceful_shutdown:
  cause: api:application-lifecycle installs no signal handler under the tinygo build tag, because TinyGo os/signal replaces the default disposition and delivers nothing
  consequence: docker stop and a Kubernetes pod deletion send SIGTERM, the process ignores it, and the grace period ends in SIGKILL every time
  what_is_lost: the shutdown_timeout drain of data:server-runtime-config, so in-flight requests are cut rather than finished
  mitigations:
    - a load balancer that stops sending new requests before the stop signal, such as a Kubernetes preStop sleep, which converts the cut into a drain the platform performs
    - a shortened grace period, since waiting for a signal nothing answers only delays the kill
  not_fixable_in_the_framework: the caller's context is the only shutdown trigger the platform offers, and a container gets no chance to cancel it
migrations_are_a_child_process:
  cause: decision:migration-execution-split delegates a TinyGo apply to pw, since system:goose is host-only and cannot be linked
  consequence: a policy:container-runtime-image image carries no pw, so api:migration-runner Migrate fails inside it with the missing-binary error rather than applying anything
  not_normally_reached: policy:migration-safety disables startup apply outside development, so a production container never calls it
  operator_path: run api:cli-migrate as a separate step from an image that carries pw and the migrations directory, which is the same step a host Go image should take
scheduler:
  rule: a project whose requirement:database-engine-selection engine speaks a network protocol builds with -scheduler=threads
  container_specific: nothing; the flag is the same one rule:tinygo-runtime-compatibility requires everywhere, and the Dockerfile is where it stops being optional to remember
architecture:
  host: the TinyGo build compiles for the machine running it, so an arm64 workstation produces an arm64 image
  cross: use the platform of the build itself, through docker buildx --platform or a matching runner, rather than a GOARCH the TinyGo invocation does not read the way go build does
base_image:
  builder: the published TinyGo image at decision:tinygo-042-baseline or later, which carries a compatible host Go for the rule:container-build-inputs host phase
  runtime: policy:container-runtime-image, unchanged, provided the binary links statically; verify per target rather than assume
verification: an image built from Dockerfile.tinygo serves traffic, answers requirement:healthcheck-subcommand, and is measured against the host Go image for size and for stop behaviour
```
