---
id: requirement:contrib-websocket
type: requirement
title: WebSocket Upgrade Seam
---
Popcorn Wave offers WebSocket as one callback-shaped upgrade over an external client that already spans both HTTP backends, and maintains no frame protocol of its own.

```yaml
status: proposed
package: github.com/shibukawa/popcornwave/contrib/websocket
integration: external dependency
client: github.com/fasthttp/websocket v1.5.12, a fork of github.com/gorilla/websocket
serves: decision:transport-handle-containment, which offers no raw-connection surface and names the upgrader as the seam instead
why_this_client:
  verified: 2026-08-08, by reading v1.5.12 sources
  one_package_two_paths:
    net_http: Upgrader.Upgrade(w, r, responseHeader) returns a Conn
    other_backend: FastHTTPUpgrader.Upgrade(ctx, handler) takes a callback, 'type FastHTTPHandler func(*Conn)'
    shape: a returned value against a callback, which is how the two backends differ at every other surface in this design
  identical_conn: both paths call the same newConn, so ReadMessage, WriteMessage, NextReader, NextWriter, SetPingHandler, SetReadDeadline, Subprotocol, and Close are one API across backends
  consequence: portability is already in the library; this package supplies the one shape that differs, and nothing else
surface:
  - Upgrade(w, r, func(*websocket.Conn) error) error
  - options for subprotocol, buffer sizes, and compression, passed through to the upgrader
no_facade:
  rule: the library's own Conn is what the callback receives, per the requirement:contrib-redis-valkey precedent of naming the client rather than wrapping it
  reason: the type is already identical on both paths, so a wrapper would add a maintenance surface and remove methods without buying portability
what_this_package_owns:
  upgrader_selection: the build-tagged choice between the two upgraders, so application code has one call
  origin_check:
    hazard: the upgrade request carries cookies, so an unchecked upgrade is cross-site WebSocket hijacking, which is the policy:csrf-protection hazard on a request that never reaches CSRF middleware
    library_default: same-origin when Origin is present, and accept when it is absent, which is correct for a non-browser client and is not a decision to leave at its default by accident
    owned: CheckOrigin is wired to this framework's request-origin resolution, including trusted-proxy handling, rather than left nil
    shape: the two upgraders take their own backend's request type here, so this is the second thing that must be supplied per build
  lifetime: the callback runs after the handler returns on a backend that registers it, so the callback closes over what it needs and never over w or r, per rule:transport-handle-checks PW0602
  close: the connection is closed when the callback returns
dependency_cost:
  fact: contrib has no module of its own, so this adds github.com/valyala/fasthttp to the go.mod of every project depending on popcornwave, whether or not it opens a socket
  linking: unaffected; an unimported package is not linked, so binary size and compile time do not move
  precedent: go-redis and klauspost/compress already sit there on the same terms, so this is existing practice rather than a new concession
  optics: the module graph of a net/http-only project would then name fasthttp, which reads oddly against the published positioning; a separate module for this package is the alternative, at the cost of a release cadence nothing else here has
tinygo_matrix:
  status: unverified and at risk
  reason: the dependency chain pulls fasthttp, klauspost/compress, and unsafe-backed byte-to-string helpers, which is the class policy:contrib-compatibility bounds
  resolution: either verify the matrix or exempt this package explicitly, the way decision:host-tools-target-runtime and decision:devidp-scope-reduction exempt others; an unstated exemption is the outcome to avoid
boundaries:
  - no frame protocol, no hub, no room or broadcast facade
  - no reconnection or message-delivery guarantees, which belong to the application
  - decision:live-delivery-transport keeps carrying live deliveries as NDJSON over an ordinary response, and this package does not become its transport
  - api:typed-stream stays the surface for anything that keeps HTTP framing
```
