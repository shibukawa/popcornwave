---
id: flow:handler-registration
type: flow
title: Handler Registration
---
A handler package owns one mux and lets each handler source register its own literal route during package initialization.

```yaml
package_surface:
  mux: var mux = pw.NewServeMux()
  accessor: Handlers() *pw.ServeMux
handler_file:
  - define request input types privately beside the handler by default
  - register a literal method-and-path pattern in init()
  - implement a standard func(http.ResponseWriter, *http.Request)
rules:
  - no central explicit handler list
  - route discovery follows rule:static-route-discovery
  - middleware remains outside the mux
  - package initialization registers routes before api:application-lifecycle starts
```
