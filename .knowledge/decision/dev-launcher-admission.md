---
id: decision:dev-launcher-admission
type: decision
title: Development Launcher Admission
---
A floating control on the application page is admitted when it is a link to requirement:dev-console and nothing more, which is what keeps the debug toolbar requirement:dev-error-overlay refuses still refused.

```yaml
status: accepted
context:
  - policy:dev-console-boundary admits one development behavior into the application, the reserved pwdev build mode, and refuses every development route that would answer with application state
  - requirement:dev-error-overlay lists a debug toolbar on the application page among its non_goals, and a floating control in the corner of that page is the shape a debug toolbar arrives in
  - the console address is printed once at startup and is gone by the time it is wanted, which is usually several rebuilds later
question: whether requirement:dev-console-launcher is the refused toolbar under a smaller footprint
decision: admitted, on the ground that it carries no answer of its own
the_line:
  admitted: a control whose whole behavior is navigation, plus a status it already receives on a stream opened for another reason
  refused: a control that answers a question about the application — a request list, a query count, a route table, a session, a merged configuration view
  test: if the control needs a fact the page does not already hold, it belongs in a requirement:dev-console pane, and the pane is what the launcher opens
why_it_holds:
  no_route: the application serves nothing new; the launcher reads the console's own stream, which terminates at the console per flow:dev-overlay-delivery
  no_state: it holds nothing about the application, so there is nothing for a build-mode mistake to disclose
  no_second_cost: the module, the console address, and the stream all exist for requirement:dev-error-overlay, so what is added is a shadow root and an anchor
  same_constraint: it lives under the pwdev build mode, so decision:dev-browser-runtime-scope bounds it exactly as it bounds the overlay
alternatives:
  printed_url_only:
    rejected: the URL is one line in a terminal that a developer scrolls past and a restart pushes further away
    kept: it is still printed; the launcher is the second way in, not a replacement
  browser_extension:
    rejected: another thing to install, and a framework that assumed one would work differently for the developer who did not
  keyboard_shortcut_only:
    rejected: undiscoverable, and it would collide with whatever the application or the browser already binds
  console_index_open_at_startup:
    rejected: a tab opened without being asked for is what requirement:dev-console already declines under its non_goals
consequences:
  - the application page now carries a visible framework-owned element in development, which is a thing to justify each time one is proposed rather than a precedent that opens the corner
  - a proposal to put anything else in that corner is measured against the test above, not against the fact that the corner is already occupied
```
