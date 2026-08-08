---
id: requirement:dev-api-reference
type: requirement
title: Development API Reference
---
The requirement:dev-console index links the API documentation UI the application already serves, resolving its path from configuration rather than assuming one, and hosts no renderer of its own.

```yaml
audience: actor:application-developer
pane_of: requirement:dev-console
supersedes: the earlier reading in which the console embedded a renderer, which was written before the application endpoint was found to exist
existing_endpoint:
  owner: policy:operational-endpoints
  selection: data:server-runtime-config api_doc, which names scalar or swagger and disables the endpoint when empty
  path: data:server-runtime-config api_doc_path, defaulting to /docs
  scaffold: api:cli-init writes api_doc into the development configuration only, so staging and production omit the key and register no route
why_a_link:
  - the application already renders the document, so a second renderer would be a second thing to keep current with the same specification
  - a link cannot disagree with what the application serves, because it is what the application serves
  - the console keeps no third-party bundle for this, which is one less license to verify and one less committed build to refresh
path_resolution:
  rule: the link is built from the resolved api_doc_path and never from the default
  reason: the path is configuration, so a console that hardcoded /docs would send the developer to a 404 in every project that moved it
  source: the same best-effort read of the development configuration that resolves the application address for the index
  precedence_limit: an environment variable or a flag outranks the file and is not consulted, so the resolved path can be wrong in a project that overrides it that way
  origin: the announced application address once there is one, since the page is a path on the application's own origin and decision:development-port-shift can move it
  undetermined: an unreadable value is reported as undetermined rather than replaced by the default, per policy:dev-console-boundary
states:
  configured: the index links the path, beside the application address it is relative to
  disabled: the index says the endpoint is off and names api_doc as the key that enables it
  application_down: the link is shown and does not work, which is the ordinary state of every link to a stopped application and is not worth a second mechanism
non_goals:
  - hosting a renderer, embedding one in a pane, or committing a bundle for one
  - sending, replaying, or recording requests, which is why this was never a client
  - showing the specification document itself; the application serves that too, at data:server-runtime-config openapi
  - a route table view, which waits on data:route-table and belongs beside api:cli-doctor
acceptance:
  - a project with a moved api_doc_path is linked at the moved path
  - a project with api_doc unset is told which key enables the endpoint
  - the console embeds no renderer and fetches no document
  - a path the console could not resolve is named as undetermined
```
