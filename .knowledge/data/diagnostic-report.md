---
id: data:diagnostic-report
type: data
title: Diagnostic Report
---
The report requirement:project-diagnostics produces: one state view per subsystem, then the findings, in a shape a person reads at a glance and a pipeline parses.

```yaml
layout:
  order:
    - summary: the diagnosed token, the framework version, the host mode, what is enabled in one line each for database, session, auth, assets, and pages, and the counts of error, warning, and note findings
    - configuration: every binding grouped as a policy:startup-summary tree, with the place marked only where it is not the default
    - features: one line per framework feature, state first
    - middleware: the effective chain in order, per data:middleware-runtime-config
    - database: the data:database-connection-set groups, connections, and resolved pointers
    - routes: the data:route-table entries, with framework mounts marked
    - registrations: what the import graph links
    - project: the api:cli-doctor project, capability, generated, migration, and development-only sections
    - findings: most severe first
    - limits: what this analysis could not determine or did not run
  rationale: state before findings, because a finding is only readable next to the value that produced it
  summary_purpose: the opening block is what an operator pastes into an issue, so it has to carry the environment, the version, and what was on, without the rest of the report
feature_entry:
  name: the feature as an operator names it, such as session, csrf, response compression, query diagnostics, OTLP export
  state: on, off, unavailable, or undetermined
  decided_by: the key that resolved the state, and the place it came from
  implementation: the linked plugin import path, contrib requirement, or driver behind it
  detail: the few resolved values that change what the feature does, not the whole binding
  unavailable: configuration selects it while the import graph links no implementation
  undetermined: the deciding key's value is not determinable from this host
binding_entry:
  prefix: the configbind prefix
  owner: framework, a named plugin import path, or application
  source: the generated configbind metadata the prefix came from
  fields: key, redacted value, and place
  unclaimed: a configured key no discovered binding owns, carried with the plugin that would own it when the analysis knows one
connection_entry:
  label: the data:database-connection-set group#ordinal label, so a finding names the same connection the logs will
  driver: DSN scheme and the linked driver answering it, or the missing driver package
  role: readonly, writable, and which of the default, write, migration, and session pointers select it
  pool: the resolved bounds, shown because a zero value is database/sql semantics rather than nothing
registration_entry:
  kind: config binding, session backend, framework extension, or database/sql driver
  package: the import path that registers it
  edge: the blank import or ordinary import that reaches it, so a surprising dependency is traceable
  selected: whether configuration selects it
finding:
  id: the data:diagnostic-check identifier, printed first, as in "PW0412: session secret is set from config file (prod)"
  docs: the documentation link that identifier resolves to
  severity: error, warning, or note
  environments: the tokens the check applies to
  message: what is wrong, in one sentence naming the resolved value
  evidence: the key and place, the missing import, the unclaimed section, the source position, or the key pair a cross-environment match found
  secret_evidence: for a secret-classified field, the key, the file, and whether that file is tracked in version control, never the value
  remedy: the command, import line, or key that resolves it
  reference: the policy or decision that owns the constraint
limit_entry:
  subject: the key, the registration, or the check group
  reason: the value arrives from the deployment's environment or CLI, the registration argument is not statically resolvable, an input such as data:route-table is stale, or the online option was not given
  effect: which checks were suppressed, so a silent skip is still visible
  principle: a report that looks clean because it did not look is the one failure mode this section exists to prevent
multi_environment:
  trigger: api:cli-doctor diagnoses more than one token
  configuration: one column per token, showing only the keys that differ between them; the full tree is rendered only for a single-token run
  absent_key: a key one token reports and another hides, because a disabled parent made it irrelevant, shows as absent rather than as a value
  reason: the question a repeated run asks is which value changes when the environment does
  features_and_findings: repeated per token, each under its own header
  secrets: a secret-classified literal shared by two tokens is reported as a match between their keys, with no value on either side, per rule:configuration-advisories
formats:
  text:
    stream: stdout
    grouping: the policy:startup-summary tree rules for dotted keys and alignment
    color: ANSI only on a terminal without NO_COLOR and TERM=dumb, and severity is also carried by a word so a pipe loses nothing
    density: a feature that is off collapses to one line, so the report length tracks what is enabled
  json:
    stability: section and field names are a supported interface, since a release gate asserts on them
    shape: keyed by diagnosed token, each holding the sections and its findings array in rendered order
    numbers: durations as Go duration strings, matching the configuration they came from
rules:
  - every value is redacted before it enters the report, per policy:log-emission and the data:database-connection-set DSN rules
  - a secret field appears as a redacted value rather than being omitted, so its presence stays visible
  - a ${NAME} expansion is reported as the reference, never expanded, because doctor is not the process that holds the secret
  - an undeterminable value is stated as unknown at this host and never rendered as its default, per decision:host-side-diagnostic-analysis
  - a section the analysis could not complete is named with its reason rather than omitted
```
