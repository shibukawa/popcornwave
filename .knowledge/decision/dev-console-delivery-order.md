---
id: decision:dev-console-delivery-order
type: decision
title: Development Console Delivery Order
---
Deliver the requirement:dev-console listener and the panes needing no new mechanism before the two panes that introduce a process and a protocol.

```yaml
status: accepted
principle: each step ends with something usable, and no step is written against a mount point or a convention the next step moves
settled_first:
  reason: these are public surface or an outside dependency, and changing either after a project has adopted it costs a migration rather than an edit
  decided:
    console_port: dev.console.port defaults to 18081 and api:cli-init scaffolds it, beside the 18080 dev.idp.port already takes
    otel_port: dev.otel.port narrows to the OTLP receiver alone, changing what an existing value means without changing its spelling, which the first release note has to say plainly
  closed:
    api_renderer: no renderer is taken and no license has to be verified, because requirement:dev-api-reference became a link to the endpoint the application already serves
order:
  - step: console listener, dev.console configuration, and index
    delivers: one bookmarkable URL
    includes: moving the requirement:dev-telemetry-viewer UI onto it per decision:dev-console-consolidation, before any pane is written against the old mount
  - step: data:dev-loop-state published by api:cli-dev and shown on the index
    delivers: the loop's current failure readable outside the terminal
    why_here: it is the whole overlay protocol with no browser code in it, so flow:dev-overlay-delivery is proven before decision:dev-browser-runtime-scope is exercised
  - step: requirement:dev-asset-inspector
    delivers: the first pane
    why_here: static analysis only, so the pane conventions of index entry, disabled reporting, and undetermined-value reporting are settled where a mistake is cheap
  - step: requirement:dev-error-overlay browser half
    delivers: the failure over the page
    depends_on: the pwdev script set of requirement:framework-script-assets
  - step: requirement:dev-api-reference
    delivers: the generated OpenAPI document, readable while the application is down
    independent: needs no mechanism the earlier steps built, so it may land beside any of them
  - step: requirement:template-storybook
    delivers: templates rendered in isolation
    introduces: decision:dev-harness-process, and the generated pwdev registry that makes an unexported symbol enumerable from inside its own package
  - step: requirement:dev-query-runner and requirement:dev-table-viewer
    delivers: declared statements run with their own types, and the tables they changed read beside them
    introduces: decision:dev-application-attachment
    order_within: the attachment and the read-only half first, because browsing exercises the transport with nothing that can write, and the executing half lands on a proven one
    why_last: it reuses the registry technique the storybook step builds, and it is the step whose conventions benefit most from a settled console
spikes:
  reason: each proves an assumption a whole step rests on, and each is cheaper than the step it guards
  items:
    - the requirement:framework-script-assets core dynamically importing a dev module under pwdev, with the revision digest separating the two URLs, which requirement:dev-error-overlay assumes
    - pw building and running a generated main inside the project module, which decision:dev-harness-process assumes
    - an application dialing out and holding an attachment across its own restart, which decision:dev-application-attachment assumes
configuration_growth:
  rule: a dev.console pane key is accepted only from the step that delivers its pane, because data:project-config treats an unknown key as an error
  effect: the schema there is the target; a key for a pane not yet delivered is rejected by name, which is a clearer answer than accepting a setting that does nothing
api_reference_supersession:
  finding: the application already serves a Scalar or Swagger UI through data:server-runtime-config api_doc, scaffolded into the development configuration by api:cli-init
  resolved: link it rather than duplicate it, which is what requirement:dev-api-reference now specifies
  effect: the step shrank from embedding a renderer to resolving api_doc_path and putting a link on the index, and it no longer carries a third-party bundle
acceptance: each requirement carries its own acceptance list, which is the step's test plan rather than a separate artifact
```
