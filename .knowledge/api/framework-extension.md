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
slots:
  SlotSession: resolves stored session state
  SlotAuthentication: finalizes data:request-authentication and owns its own login paths
  SlotGuard: rejects unauthenticated requests to protected paths
rules:
  - registration completes before ParseConfig
  - reject duplicate extension names
  - Setup runs once per framework initialization, after configuration parsing and database startup
  - Setup receives the same data:request-context-capsule resources handlers will see
  - Setup runs in ascending slot order so a later slot may read earlier prepared state
  - the chain is composed so the lowest slot is outermost and every extension sits inside resource injection
  - a nil middleware installs nothing, which is how a disabled capability opts out
  - Setup failure is a startup error, never a first-request error
  - Close is registered once per name and runs in reverse order during shutdown
boundaries:
  - core pw imports no extension package, so only linked capabilities contribute code and configuration
  - extensions own their own paths by interception; there is no framework route registration API
consumers:
  - plugin/auth registers the session, authentication, and guard extensions
```
