---
id: data:framework-metric-set
type: data
title: Framework Metric Set
---
The instruments requirement:framework-metrics records, each named by the seam that already produces its value, so a reader sees which are the OpenTelemetry floor, which only this framework can see, and which cannot be measured at all from where the framework sits.

```yaml
configuration: data:observability-runtime-config observability.metrics
collection: flow:metric-collection
scope: github.com/shibukawa/popcornwave, matching data:framework-span-set
convention:
  duration_unit: s, float64 histogram, per semantic convention, even where a span reports nanoseconds
  size_unit: By
  count_unit: the counted thing in braces, as {request} or {connection}
  attribute_names: the semconv spelling where one exists, and the pw prefix of data:framework-span-set where none does
http_server:
  group: http, part of the semconv floor
  source: the request span of requirement:contrib-otel, recorded by the same middlewares Otel frame at the deferred end where it already reads the status
  instruments:
    http.server.request.duration:
      kind: histogram
      unit: s
      attributes: http.request.method, http.response.status_code, http.route, url.scheme, error.type when the request failed
      answers: request count, requests over time, average, and percentile, all from one instrument
    http.server.active_requests:
      kind: up_down_counter
      unit: "{request}"
      attributes: http.request.method, url.scheme
      answers: concurrency, which no per-request record shows and which is what a saturated instance looks like before it looks slow
    http.server.request.body.size:
      kind: histogram
      unit: By
      note: the declared length; a chunked request has none and records nothing rather than zero
    http.server.response.body.size:
      kind: histogram
      unit: By
      source: the byte count middlewares ResponseTracker already accumulates, which is also what pw.render.bytes reports uncompressed
http_client:
  group: http
  source: contrib/otel/otelhttp Transport, which already opens the client span and knows the outcome
  instruments:
    http.client.request.duration:
      kind: histogram
      unit: s
      attributes: http.request.method, server.address, server.port, http.response.status_code, error.type
      cardinality: no url.path, for the reason the transport's own span name comment records about a path-per-object API
    http.client.active_requests:
      kind: up_down_counter
      unit: "{request}"
  opt_in: the transport takes a meter provider explicitly rather than reaching for the process one, because a client this framework did not build is one whose call sites it cannot vouch for; a caller that knows its endpoints are bounded passes WithMeterProvider
  excluded_caller: the exporter's own client, which is already excluded from propagation by requirement:contrib-otel and would make exporting a metric record a metric
db_client:
  group: db, part of the semconv floor
  source: decision:executor-seam-instrumentation, the resolver seam api:instrumented-sql-executor already wraps
  instruments:
    db.client.operation.duration:
      kind: histogram
      unit: s
      attributes: db.system.name as the driver, db.operation.name as the statement keyword from the closed allowlist, pw.db.connection as the pool label, error.type
      answers: query count, query duration, and slow-tail shape per driver and keyword
      excluded: db.query.text, which is unbounded and belongs on the span where observability.trace.statement already bounds it
  pool:
    source: sql.DB.Stats on each pwruntime Connection, read by an observable callback per interval rather than on the request path
    instruments:
      db.client.connection.count:
        kind: up_down_counter
        unit: "{connection}"
        attributes: db.client.connection.state of used or idle, and db.client.connection.pool.name
        from: DBStats InUse and Idle
      db.client.connection.max:
        kind: up_down_counter
        from: DBStats MaxOpenConnections
      pw.db.connection.waits:
        kind: counter
        from: DBStats WaitCount
        why_not_db.client.connection.timeouts: that name means the waits that timed out and database/sql does not distinguish them, so reporting waits under it would overstate a failure that may not have happened; the pw name says what it counts
      pw.db.connection.wait.time:
        kind: counter
        unit: s
        from: DBStats WaitDuration, the accumulated total
    why_it_matters_here: an exhausted pool makes every request slow with no statement being slow, which is the one failure the per-statement histogram above cannot show
  unavailable:
    db.client.response.returned_rows:
      why: api:instrumented-sql-executor states it — the query method returns the concrete rows value, which cannot be decorated, so row count stays outside the measurement
      partial: pw.db.rows_affected exists for an exec and could carry one, which would be an instrument that means something different on half the statements
    db.client.connection.wait_time:
      why: the specification wants a histogram and DBStats offers only a cumulative total, so the distribution was never recorded and cannot be recovered
      shipped_instead: pw.db.connection.wait.time above, a counter of accumulated seconds, which answers whether the pool waits at all and not how badly any one caller did
    db.client.connection.create_time and use_time: database/sql exposes neither
go_runtime:
  group: runtime, part of the semconv floor
  source: the runtime/metrics package, read by observable callbacks
  instruments:
    go.memory.used: up_down_counter, By, attribute go.memory.type of heap, stack, other, or free
    go.memory.limit: up_down_counter, By, total obtained from the OS
    go.memory.allocated: counter, By
    go.memory.allocations: counter, "{allocation}"
    go.memory.gc.goal: up_down_counter, By
    go.goroutine.count: up_down_counter, "{goroutine}"
    go.config.gogc: up_down_counter, "%"
    go.processor.limit: up_down_counter, "{thread}"
  not_read: go.schedule.duration, which runtime/metrics reports as a histogram this package's observable form cannot carry
  why_the_group_is_separate: it is the one group with no framework seam at all, it costs a callback per interval whether or not the application is serving, and a deployment already collecting it from its own agent turns it off here
  not_the_same_as: the process samples of requirement:dev-telemetry-viewer, which watch the process from outside and cannot see a heap or a goroutine
  tinygo: the runtime/metrics surface is not the same there, so this group is host-only and reports nothing rather than approximating
render:
  group: render, framework-specific
  source: the render span of data:framework-span-set, which already holds every value below
  instruments:
    pw.render.duration:
      kind: histogram
      unit: s
      attributes: pw.render.mode from the closed set of six, and nothing else — the route is recorded by the http group, and adding it here would multiply six modes by every page for a question the two series already answer together
      answers: what each branch of decision:automatic-async-render-selection cost, and how often it was taken; nothing outside the process can attribute a response time to a render mode
    pw.render.bytes:
      kind: histogram
      unit: By
      attributes: pw.render.mode
      note: uncompressed bytes that reached the client, the same value and the same live-response rule the span attribute carries
    pw.render.cache.operations:
      kind: counter
      unit: "{operation}"
      attributes: result of hit or miss
      source: the per-response tally requirement:component-output-cache binds to the store
      shape: one counter with a result attribute rather than two instruments, because a reader dividing them needs both under one name; the reason both halves are reported at all is the one that requirement records
      only_from: a response that consulted the store, so a project writing no cache annotation produces no series rather than a flat zero
  boundary:
    instruments:
      pw.boundary.settle.duration:
        kind: histogram
        unit: s
        attributes: http.route only
        extent: the same commit-to-completion interval the await boundary span measures, which is how long a fallback held the screen
        excluded_attribute: pw.boundary.id, which is positional and safe on a span but unbounded across pages and would make one series per placeholder of every page
      pw.boundary.settled:
        kind: counter
        unit: "{boundary}"
        note: also the count of the histogram, and named separately only if a boundary that never settles must be counted somewhere the histogram cannot reach
  live:
    source: flow:live-boundary-delivery and the live delivery span
    instruments:
      pw.live.subscriptions.active:
        kind: up_down_counter
        unit: "{subscription}"
        why: policy:live-subscription-bounds bounds a thing whose lifetime the server did not choose, and every bound in it is a number an operator currently cannot see approaching
      pw.live.delivery.duration: histogram, s, the interval between consecutive deliveries of one boundary
      pw.live.delivery.bytes: counter, By
      pw.live.closed:
        kind: counter
        unit: "{response}"
        attributes: pw.live.close_reason of done or retry
        why: a retry close is the rollover the policy trades for authorization re-checks, and its rate is how that trade is watched
      pw.live.refused:
        kind: counter
        attributes: which bound refused, from the closed set of per_response_boundaries and per_session_responses
        why: the understated weakness that policy records — an anonymous bound behind a proxy refuses ordinary traffic — is invisible until refusals are counted
data_cache:
  group: cache, framework-specific
  source: pwruntime CacheStore.Stats, read by an observable callback per configured store of data:cache-store-set
  instruments:
    pw.data_cache.operations:
      kind: counter
      unit: "{operation}"
      attributes: result of hit, stale_hit, miss, or coalesced, and the store name
      closes: the not_built line of requirement:data-result-cache, which built the four counters and left them unreadable outside the process
    pw.data_cache.entries: up_down_counter, "{entry}", the store name; how near the configured cap a store runs, which is what makes an entry-per-reader private cache visible before it thrashes
  no_ratio: hit rate is these terms divided by whoever reads them, per requirement:framework-metrics
candidates:
  purpose: instruments with a real seam and no decided owner, listed so the first version can decline them explicitly
  pw.action.duration:
    kind: histogram
    attributes: the declared action name, which api:server-action fixes at generation and is therefore a closed set
    open: whether an action is the framework's to time or the handler's, which requirement:modern-observability currently answers the second way
  pw.ratelimit.decisions:
    kind: counter
    attributes: result of admitted, refused, or degraded
    source: api:rate-limit-limiter Admit, whose degraded state its own requirement calls the state worth knowing about
  authentication_outcomes: a counter per outcome class, which needs a closed outcome set nothing has declared yet
excluded:
  - a metric per component instance or per boundary id, for the cardinality reason above
  - a metric carrying a trace id, which is a per-request identifier wearing an aggregate's clothing
  - exemplars, per requirement:framework-metrics
  - the http cache layer, which policy:layered-cache puts in front of the process
  - the exporter's own dropped-record count, which flow:telemetry-export keeps as a log
cost:
  disabled: no instrument object, no observable registration, and one nil comparison per response and per statement, matching data:framework-span-set
  enabled_per_request: one histogram record per request and per statement, which is an attribute-set lookup and an add rather than an allocation per record
  enabled_per_interval: one callback per pool, per store, and one for the runtime group
```
