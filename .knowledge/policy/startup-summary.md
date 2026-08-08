---
id: policy:startup-summary
type: policy
title: Startup Summary
---
Resolved configuration is reported once per process, in the shape its reader can use, instead of one record per key.

```yaml
capture:
  when: api:runtime-configuration finishes loading data:loaded-configuration
  content: environment, resolved config path, every key with value and winning place, start time
  secrets: policy:log-emission redaction applies before the value is stored, and a DSN takes the rule:dsn-redaction form rather than the whole mask
emission:
  once: the first of Run or Middlewares to complete initialization emits it
  run: emitted after the listener binds, so the reported address is the accepted one and not the configured one decision:development-port-shift may have moved off
  middlewares: emitted after initialization without a listening address
selection:
  key: observability.boot_log
  auto: tree when stderr is a character device, otherwise record
  tree: banner, configuration grouped as a tree, and the listening URL, written to stderr
  record: one api:logger record named "popcornwave started"
  off: nothing
tree:
  grouping: dotted keys nest by section; values align with their siblings
  provenance: only non-default places are marked (file, env, flag)
  color: ANSI only on a terminal without NO_COLOR and TERM=dumb
record:
  config: nested groups mirroring the key structure
  config_source: flat group naming the place of every non-default key
  listening: present only when the framework owns the listener
rationale:
  - one boot event keeps log collectors and OTLP exporters from ingesting a record per key
  - a tree shows an operator the shape of the configuration and what was overridden
```
