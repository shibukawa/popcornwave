---
id: api:cli-check
type: api
title: petitweb check
---
petitweb check verifies configuration, generated artifacts, host tests, and TinyGo compilation without producing a release artifact.

```yaml
usage: "petitweb check [--skip-tinygo]"
checks:
  - parse and validate data:project-config
  - confirm required tools and decision:tinygo-042-baseline
  - run api:cli-generate --check
  - run "go test ./..."
  - compile configured package to a temporary output with TinyGo
  - compile imported contrib packages and run their target matrix smoke tests
reporting:
  - ordered phase labels
  - exact failing command
  - remediation for missing or incompatible tool
exit:
  all_checks_pass: 0
  any_check_fails: nonzero
```
