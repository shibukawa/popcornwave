---
id: data:diagnostic-check
type: data
title: Diagnostic Check
---
One check has a stable identifier, a severity, declared inputs, and a documentation page, so a finding can be searched, cited in an issue, and read about.

```yaml
identity:
  form: PW followed by four digits, such as PW0412
  stability: an identifier is never reused or renumbered; a retired check keeps its page and is marked retired
  ranges:
    PW01xx: rule:project-integrity-checks
    PW02xx: rule:route-and-template-checks
    PW03xx: rule:storage-checks
    PW04xx: rule:configuration-advisories, including wiring, secrets, and the identity provider
    PW05xx: rule:production-readiness-checks
    PW06xx: rule:transport-handle-checks, the first range whose runner is an analyzer rather than api:cli-doctor
fields:
  id: the PW identifier
  title: the one-line form that appears in the report, such as "session secret is set from config file"
  severity: error, warning, or note, resolved per diagnosed token by the owning catalog
  scope: the data:runtime-environment tokens the check applies to
  inputs: which of merged configuration, import graph, project files, data:route-table, typed application Go syntax, process environment, or network the check needs
  phase: startup, doctor, vet, or a combination, per decision:shared-check-catalog
  remedy: the command, import line, or key that resolves it
  docs: one page per identifier
documentation:
  form: one generated reference page, with a heading and an anchor per identifier
  url: the page anchor derived from the identifier and title, so a finding links to its own entry
  generation: the page is generated from the catalog and checked in; a test compares the two, so a check added without documentation fails the build
  content: severity, the environments it applies to, what it reads, and the remedy, which is what a troubleshooting page has to say anyway
  reason: an identifier in a terminal is only useful when it is searchable, and generating from the catalog is what keeps that true
reporting:
  finding: data:diagnostic-report carries the identifier, the title, the evidence, and the documentation link
  message: "PW0412: session secret is set from config file (prod)" is the intended shape, identifier first
  issue_reports: an operator pasting one line carries the check identity, the environment, and the framework version with it
rules:
  - a check declares its inputs, and a runner skips it rather than guessing when an input is unavailable
  - a check states one condition; a check that would report two conditions is two checks with two identifiers
  - severity is data on the check, not a decision made at the call site
  - the identifier is assigned when the check is written, not when it fires
```
