---
id: policy:container-runtime-image
type: policy
title: Container Runtime Image
---
The runtime stage of a Popcorn Wave image carries the application binary, its environment configuration file, and nothing else, so the image has no shell, no package manager, and no build tool to be reached through.

```yaml
base:
  selected: gcr.io/distroless/static-debian13:nonroot
  provides: CA certificates for requirement:contrib-oidc discovery and any outbound HTTPS, tzdata, /etc/passwd entries, and no shell
  requires: a statically linked binary, so CGO_ENABLED=0 for host Go
  scratch_rejected: an OIDC or OAuth login fails at the TLS handshake with no certificate pool, and the failure names the peer rather than the missing file
  cc_variant: gcr.io/distroless/base-debian13 is the fallback for a binary the selected toolchain does not link statically
user:
  runs_as: nonroot, uid 65532
  port: the data:server-runtime-config port stays above 1024 so the unprivileged user can bind it
  filesystem: read-only is supported, because requirement:public-asset-delivery serves the embedded tree and public.read_local defaults to false
configuration:
  file: config.prod.toml copied into the working directory, resolved by policy:config-file-resolution against the process working directory
  workdir: set explicitly, since project-local resolution has no other base and an unset WORKDIR makes / the search root by accident
  environment: APP_ENV is set in the image, because requirement:environment-switching treats an unset token as dev and would load the development file or none
  port_override: PORT is honoured for a platform that assigns one, per data:server-runtime-config
  secrets:
    rule: a DSN, a client secret, a keyring secret, or a cookie store secret is supplied by the environment at run time and never copied into a layer
    reason: an image layer is readable by anyone who can pull the image, and deleting the file in a later layer does not remove it
    scaffold: config.prod.toml carries structure and non-secret defaults only
probe:
  instruction: HEALTHCHECK in exec form calling requirement:healthcheck-subcommand on the application binary itself
  reason: the image has no curl, no wget, and no shell to run either with
  requires: server.health set in the environment configuration, or the probe exits 1 naming the key
excluded_from_the_runtime_stage:
  - the Go toolchain, system:pw-cli, and the Tailwind executable, all of which belong to the builder stage of rule:container-build-inputs
  - the migrations directory, because policy:migration-safety disables startup apply outside development and api:cli-migrate runs as a separate step
  - requirement:contrib-devidp and any development-only package, which api:cli-build already refuses to link
  - the public source tree, which is embedded rather than read from disk
logging:
  format: data:observability-runtime-config stdout_format json in a container, against the plaintext default api:cli-init writes into config.dev.toml
  destination: stdout only; the runtime writes no log files and the image provides nowhere to put them
```
