---
id: decision:development-port-shift
type: decision
title: Development Port Shift
---
A development run that cannot bind the port data:server-runtime-config names walks forward to the next one it can, and says so; every other environment fails instead.

```yaml
status: accepted
problem:
  symptom: api:cli-dev reports "application exited: listen tcp :8080: bind: address already in use" and then waits for a change that will not fix it
  causes:
    - a second project, or a second copy of the same one, open at the same time
    - a leftover process from a loop that did not unwind, which api:cli-dev already records as a known shape
  cost: nothing is served, and the developer has to find and stop the holder before any work continues
decision:
  development: bind the configured port, and on failure try each of the next ten in turn
  elsewhere: bind what was configured and return the failure
  gate: data:runtime-environment development, which is APP_ENV=dev and the unset default it resolves to
  bound: ten ports, so a machine with a wide range taken reports the configured port rather than answering somewhere nothing is pointed at
  exhausted: the first failure is what the caller sees, because that is the port the developer asked about
why_development_only:
  address_is_a_contract: a health check, a reverse proxy, and an operator all go to the port the configuration names, so a deployment answering elsewhere is worse than one that does not start
  nothing_was_told: a development listener has no dependant outside the machine, and a second copy of an application is ordinary there
  unset_environment: an unset APP_ENV resolves to development, so a deployment that forgot the variable can shift; the warning names the environment, and policy:startup-summary already reports the configured value beside the accepted address
why_not_classified:
  behavior: the walk does not ask why the first bind failed
  reason: the address is a wildcard with a configured port, so what can go wrong is the port being held or being privileged, and both mean this port cannot be served
  portability: Windows reports a winsock number that syscall.EADDRINUSE does not match, so classifying would leave the shift dead on a platform it was asked for
reporting:
  warning: names the configured port, the port bound, the failure that caused the move, the environment, and that only a development run does this
  summary: policy:startup-summary already carries both halves — the configured value under its key, the accepted address as the listening URL
  console: the application announces the address it bound, per decision:dev-application-attachment, because everything requirement:dev-console can work out on its own comes from a file the shift outranks
alternatives:
  reserve_a_port: rejected; a reserved port moves every run, and the address a developer bookmarks would never be the one in the project file
  pw_dev_probes_and_injects: rejected; api:cli-dev reads the development configuration best effort only, so it would override a port set by an environment variable, a flag, or a file it does not read
  configuration_key: not taken; the environment already decides this, and a key would be a second answer to the same question
scope:
  applies: api:application-lifecycle Run, which is the path that owns a listener
  excluded: api:application-lifecycle Middlewares, where the application binds its own listener and the framework never learns the address
  excluded_ports: requirement:dev-console, requirement:dev-telemetry-viewer, and requirement:contrib-devidp keep fixed ports; each is bookmarked or built into an issuer, and each already has its own answer to a collision
```
