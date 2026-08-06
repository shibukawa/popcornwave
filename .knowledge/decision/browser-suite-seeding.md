---
id: decision:browser-suite-seeding
type: decision
title: Browser Suite Seeding Route
---
A Playwright-style browser suite seeds and asserts data:seed-dataset files through in-process HTTP endpoints (api:dev-test-endpoints) served by the pwdev application itself, not through a CLI subprocess or an external sidecar.

```yaml
status: accepted
scope: requirement:test-data-seeding
problem:
  - a browser suite is a separate process, so api:test-seed (in-process Go) cannot serve it
  - api:cli-seed resolves its DSN by compiling and running the application, costing seconds per call
  - middleware.rdb connection DSNs have no environment variable form, so a suite cannot redirect a CLI call by env alone
  - an in-memory SQLite pool is unreachable from any second process
choice:
  - mount seed and assert handlers on the application listener under the reserved /_pw namespace, pwdev build only
  - reuse internal/dbseed, keeping decision:dbtestify-integration the single dbtestify boundary
  - target the running pool itself, so the seeded database is exactly the one handlers read
rejected:
  cli_subprocess:
    idea: the suite shells out to api:cli-seed between tests
    reason: per-call application compile, and no reach into :memory: pools
  dbtestify_http_sidecar:
    idea: run the upstream dbtestify http server beside the suite
    reason: the DSN is written twice and drifts; a second pool races the application's on file-backed SQLite
  testutil_bridge:
    idea: expose api:test-run over a port for external clients
    reason: the bridge exists to copy configuration for Go tests; a browser suite needs the developer's real server
consequences:
  - api:cli-dev is the natural server for a browser suite; a release binary carries no endpoint bytes
  - suite writes are committed, so isolation comes from reseeding, not from api:test-run WithTransaction
```
