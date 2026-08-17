---
id: flow:telemetry-export
type: flow
title: Telemetry Export Flow
---
Telemetry export batches completed spans and log records within bounded memory and never blocks application shutdown indefinitely.

```yaml
flow:
  trigger: requirement:contrib-otel emits a completed span or log record
  steps:
    - id: enqueue
      action: append to a bounded queue
      failure: increment dropped-record count without blocking
    - id: batch
      action: flush on maximum batch size or interval
    - id: encode
      action: encode OTLP JSON Protobuf mapping
    - id: send
      action: POST OTLP/HTTP traces to /v1/traces and logs to /v1/logs with the configured http.Client
    - id: retry
      action: retry transient failures with bounded exponential backoff and jitter
      exhausted: account for dropped records and expose exporter error without stdout fallback
  shutdown:
    - reject new records after closing starts
    - flush within context deadline
    - return final exporter error
routes:
  purpose: where the POST above lands, which is one endpoint to this flow and two very different deployments
  relayed:
    shape: the process exports to a collector — a sidecar, a node agent, or a gateway — which forwards to the backend
    keeps: tail sampling, retry and buffering outside the process lifetime, resource attributes added downstream, and one place holding the backend credential
    sampling: the process may record everything, because the stage that can see a finished trace is the stage that decides what to keep
  direct:
    shape: the process exports to the collection backend itself, with no stage in between
    buys: one less component to run, and a developer loop and a deployment that differ only in the address
    costs: every span is billed and shipped from the process, there is no tail stage, a retry exhausted here is a record lost rather than deferred, and the backend credential lives in otel.headers of every instance
    sampling: head only, per requirement:trace-head-sampling, which is why decision:sampling-default-follows-the-environment stopped defaulting to always_on
  both_supported: the two are configurations of one exporter and differ in the endpoint, not in this flow's steps; nothing below the send step knows which route it is on
  local_receiver: requirement:dev-telemetry-viewer is the relayed shape with the relay being a viewer that forwards nowhere, which is why its plaintext exception below is a boundary rule rather than a route
security:
  - policy:outbound-transport-security applies; verified HTTPS is required outside the local proxy boundary
  - configurable headers are copied and secrets are not logged
  - response bodies and diagnostic text are size bounded
```
