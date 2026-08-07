---
id: decision:development-session-modes
type: decision
title: Development Session Intent Modes
---

Development configuration names intent rather than storage mechanics.

```yaml
modes:
  dev-volatile:
    meaning: discard session records when the process restarts
    implementation: decision:development-memory-session-backend
    default: generated development config when no session mode was explicitly selected
    cookies: opaque session token and masked CSRF token; no sealed record cookie when every protected slot is server-placed
    keyring: required only for independently signed cookie slots such as ReadOnly
  dev-persist:
    meaning: keep development session records across process restarts
    implementation: decision:cookie-session-storage
    cookies: opaque session token, sealed session record, and masked CSRF token when enabled
    keyring: stable development keyring required
constraints:
  environment: both names are rejected unless the resolved environment is dev
  explicit_choice: api:cli-init preserves an explicitly selected general backend
  general_cookie_backend: cookie remains available outside development and is not renamed
  internal_name: memory is an implementation identifier, not public configuration vocabulary
migration:
  memory: removed as a public name before release; no compatibility alias
```

`dev-volatile` favors schema and codec iteration. `dev-persist` favors authentication and workflow iteration where repeated login after each restart is disruptive. Startup summaries and generated comments state the restart behavior.

`data:session-runtime-config` owns parsing and environment validation. Storage semantics remain under `api:session-store`; handlers observe the same `api:session-manager` API in either mode.
