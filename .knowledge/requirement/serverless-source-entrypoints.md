---
id: requirement:serverless-source-entrypoints
type: requirement
title: Serverless Source Entrypoints
---
Provider source builds receive generated Google Cloud Run functions and Vercel Go entrypoints from the selected application backend.

```yaml
shared_application_boundary:
  input: the selected build-tagged application main and its application module
  output: one http.Handler or one cached initialization error
  nethttp: transform pw.Run into capture through pw.Middlewares
  fasthttp: transform pwfast.Run into capture through pwfast.Start, then expose pwfast.NetHTTPHandler
  initialization: once per warm provider instance; concurrent first requests share the result
  failure: cache the error, log it without secret values, and return a generic 500 on every request from that instance
  parity: routes, middleware order, config loading, embedded assets, operational endpoints, and security checks match the normal application
generation:
  source: api:cli-generate outputs plus data:project-config routing and main settings
  destination: .pw/build/<target>/<backend>/; generated provider files do not enter application source directories
  dependencies: the staged module pins every provider runtime library needed by its target
  configuration: config.prod.toml and required generated or embedded inputs are included; secrets remain environment variables
  module: copy the application as a nested module, create a provider module, normalize replacements, run go mod tidy and vendor, then compile with the vendor tree
google_cloud_run_functions:
  registration: register one HTTP function with the Go Functions Framework during package initialization
  target_name: stable generated identifier published beside the artifact for provider configuration
  handler: delegate directly to the shared initialized handler
  no_listener: the Functions Framework owns serving
vercel_go:
  file_shape: one Go file under api in the staged deployment root
  export: Handler with the http.HandlerFunc signature required by the Vercel Go runtime
  handler: delegate directly to the shared initialized handler
  no_listener: Vercel owns invocation
resource_lifetime:
  warm_instance: database pools, stores, telemetry, and caches are reused
  eviction: provider process termination is the cleanup boundary; request completion must not close process resources
  forbidden: request-scoped initialization or one database pool per invocation
build_contract:
  local: generation and provider source compile run before a bundle is reported ready
  remote: the provider may repeat compilation, but must not be the first place missing generated sources are discovered
  versioning: provider runtime and framework/library compatibility are recorded with the artifact
conformance:
  requests:
    - method, path, raw query, request body, host, and repeated request headers
  responses:
    - status, repeated headers, Set-Cookie, binary body, redirect, and error response
  runtime:
    - concurrent cold initialization, warm reuse, timeout cancellation, and buffered or streamed response behavior
fasthttp_bridge:
  transport: one in-memory HTTP/1 connection per invocation
  purpose: preserve method, target, headers, body, status, repeated response headers, cookies, and binary body without structural request translation
  provider_surface: net/http.HandlerFunc remains the only exported ABI
non_goals:
  - provider SDK types in application handlers
  - Google CloudEvents or another non-HTTP trigger
  - Vercel Edge runtime
  - remote deployment or project provisioning
acceptance:
  - an unchanged scaffolded application produces both staged target trees
  - each tree compiles under its provider runtime contract
  - both targets pass the same HTTP conformance fixture
  - initialization happens once under concurrent first requests
  - no secret or development-only artifact enters either staging tree
```
