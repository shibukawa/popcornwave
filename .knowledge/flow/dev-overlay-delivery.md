---
id: flow:dev-overlay-delivery
type: flow
title: Development Overlay Delivery
---
A phase transition in api:cli-dev reaches an open page through the console rather than through the application, so the path stays intact while the application is being replaced.

```yaml
trigger: an api:cli-dev phase transition that changes data:dev-loop-state
steps:
  - publish the record to requirement:dev-console, which holds the current one and nothing older
  - the console pushes it to every subscribed page over an event stream on its own listener
  - a page subscribed under a failed record renders the overlay above the document
  - a page subscribed under a healthy record whose build identity differs from its own reloads once
  - a healthy record matching the page's own build identity clears any overlay and does nothing else
  - requirement:dev-console-launcher takes its status from the same records, on the same stream, and subscribes nothing of its own
subscription:
  loaded_by: the requirement:framework-script-assets core, which dynamically imports the dev module under pwdev per decision:dev-browser-runtime-scope
  address: the console URL, baked into the module bytes the framework serves, because api:cli-dev already injects that URL into the application process
  cross_origin: the application and the console are different loopback ports, so the console answers the loopback origin; nothing else may subscribe
  transport: an event stream on the console, which reconnects on its own; that is what a page open across an application restart needs, and requirement:live-connection-recovery defines the same shape for the application's own stream
  first_record: the current state is sent before the stream waits, so a page that connects after the transition it cares about is still told about it
failure:
  console_unreachable: the page keeps its last overlay and retries; it never blanks a readable diagnostic on a transport failure
  stream_unsupported: no overlay, and no other behavior changes
  no_runtime_reference: a document shell without the requirement:external-boundary-runtime module loads no dev module, so the page is unaffected and the console index is the only report
rules:
  - the stream carries data:dev-loop-state and nothing else
  - the application process is never asked for loop state, and serves no route for it
  - a reload is issued at most once per healthy transition
  - the overlay is removed from the document when it clears, so nothing accumulates across transitions
```
