---
id: api:framework-extension
type: api
title: Framework Extension Registry
---
An imported package contributes middleware and startup work to the pw request chain without pw importing it.

```yaml
surface:
  - pw.RegisterExtension(pw.Extension) called from an imported package init
  - Extension fields are Name, Slot, Setup, and Close
  - Setup(context.Context) returns one middleware or nil
  - Close(context.Context) releases only resources the extension owns
  - pw.RegisterMiddleware(slot, name, middleware) is the application surface over the same registry, per requirement:application-middleware-registration
slots:
  line: every framework frame carries a Slot on one number line, tens taken by the framework, per requirement:application-middleware-registration
  SlotStorage: 110, installs storage clients later slots resolve
  SlotSession: 120, resolves stored session state
  SlotAuthentication: 130, finalizes data:request-authentication and owns its own login paths
  SlotCSRF: 140, rejects forged unsafe requests
  SlotGuard: 150, rejects unauthenticated requests to protected paths
  fixed: SlotOperational 100 and SlotAPIDoc 160 are handler frames and refuse registration at their exact number
rules:
  - registration completes before ParseConfig
  - reject duplicate extension names
  - Setup runs once per framework initialization, after configuration parsing and database startup
  - Setup receives the same data:request-context-capsule resources handlers will see
  - Setup runs in ascending slot order so a later slot may read earlier prepared state
  - the chain is composed so the lowest slot is outermost; an extension using the provided constants sits inside resource injection at slot 20
  - a nil middleware installs nothing, which is how a disabled capability opts out
  - Setup failure is a startup error, never a first-request error
  - Close is registered once per name and runs in reverse order during shutdown
boundaries:
  - core pw imports no extension package, so only linked capabilities contribute code and configuration
  - extensions own their own paths by interception; there is no framework route registration API
  - api:package-registration does not change this; a concept:component-package serving routes exposes a Register function the application calls, and registration installs nothing that answers a request
consumers:
  - plugin/auth registers the session, authentication, and guard extensions
  - a concept:component-package contributing middleware or startup work uses this registry unchanged; api:package-registration adds identity and assets and duplicates nothing here
  - an application inserting its own middleware uses pw.RegisterMiddleware from main, per requirement:application-middleware-registration
```
