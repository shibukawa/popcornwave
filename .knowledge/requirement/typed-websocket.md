---
id: requirement:typed-websocket
type: requirement
title: Typed WebSocket On Both Builds
---
Popcorn Web offers WebSocket as a typed socket over system:tinybind-websocket, published from pw and pwfast under one name, with the message codecs generated from the declared inbound and outbound structs.

```yaml
status: implemented 2026-08-11
supersedes: requirement:contrib-websocket, whose external client, contrib placement and raw-Conn surface are all void now that the module ships the capability
closes: the websocket entry of requirement:second-build-feature-parity class_d, which deferred this to its own session
serves: requirement:alternate-http-backend-readiness and decision:transport-handle-containment, which named an upgrader as the one seam it would admit
premise_that_changed:
  written_when: 2026-08-08, when no upgrade path existed in any dependency
  assumed: an external github.com/fasthttp/websocket in contrib, a raw *websocket.Conn handed to the callback, a new module dependency, and an unverified TinyGo matrix
  true_now: system:tinybind v0.5.7 ships the typed socket on both transports over system:tinygodriver, whose websocket packages first exist together in v1.2.3; this module requires both already, at v1.2.4
  consequences:
    dependency_cost: none; no module is added and the packages are siblings of ones already linked
    tinygo_matrix: no longer at risk, since the driver packages exist to build under TinyGo
    facade_question_answered: the callback receives a typed Socket rather than a Conn, so the no-facade argument that a wrapper would only remove methods no longer applies — the typing is the capability
    placement: pw and pwfast rather than contrib, because a contrib package cannot satisfy the rule below
placement_is_forced:
  rule: requirement:pw-call-registration and decision:transport-source-transform rewrite a handler by moving the import qualifier and nothing else
  therefore: the entry has to be spelled pw.WebSocket on one build and pwfast.WebSocket on the other, which contrib cannot supply
  also: requirement:contrib-acceptance is about optional integrations, and a socket route is framework surface an application reaches through the same generation run as every other declared type
surface:
  one_type: pw.Socket and pwfast.Socket alias the module's Socket, so a callback body is the same text on both builds, exactly as api:typed-stream established
  entries: WebSocket and WebSocketWith, plus SetSocketDefaults and SocketDefaults
  detail: api:typed-websocket
what_this_framework_owns_beyond_the_module:
  origin_check:
    what: decision:websocket-origin-check-owner
    why_it_is_the_load_bearing_one: an upgrade request carries cookies and never reaches the CSRF middleware, so an unchecked upgrade is the policy:csrf-protection hazard with a persistent connection
    precondition_now_met: requirement:proxied-request-identity shipped internal/requestorigin, which is what the module's two-string default cannot reach
  upgrade_capable_server: decision:websocket-upgrade-capable-server, which pw can take because it owns the bootstrap the module deliberately does not
  generation: the two socket call patterns registered against pw in internal/pwgen/options.go, without which a socket opens and then fails on its first message
  diagnostics: nothing new; a socket callback capturing the request is already rule:transport-handle-checks PW0604, and the callback outliving the handler on the second build is the same lifetime rule the stream carries
  documentation: a guide in both locales, positioned against api:typed-stream so a reader learns which of the two a problem wants
boundaries:
  - no hub, no rooms, no broadcast facade, and no connection registry; those are the application's, and the module's own example shows the shape
  - no reconnection or delivery guarantees
  - decision:live-delivery-transport keeps flow:live-boundary-delivery on an ordinary NDJSON response, and this does not become its transport
  - api:typed-stream stays the answer for anything that keeps HTTP framing, and the guide has to say so or every reader reaches for the socket first
acceptance:
  met_2026_08_11:
    one_source_two_builds:
      how: internal/transportfixture holds a socket handler the transform analysis admits, and both halves run the same callback body against a real handshake
      evidence: TestAuthoredHandlersCanBeRewrittenForTheSecondTransport, and TestASocketCarriesTypedMessagesBothWays in pw and in pwfast
    codecs:
      how: the generation fixture calls the entry spelling neither type argument
      evidence: TestPWCallsDriveTinyBindGeneration asserts a decoder for the inbound type, an encoder for the outbound one, and neither of the other two
    origin:
      evidence: the pwruntime socket origin tests for the judgement, and an upgrade refused 403 with the websocket_origin document on both transports
    tinygo:
      how: a socket program built with tinygo and driven over a raw socket
      observed: 101 Switching Protocols, where an unfixed bootstrap hangs with no error and no log line
      second_observation: the connection then closed 1000, because that scratch program ran no generation — which is the opens-then-dies failure the registered patterns exist to prevent, seen live rather than reasoned about
    symmetry:
      evidence: TestEveryTransportTakingPwEntryHasAPattern and TestEveryRegisteredCallHasSomewhereToLand, which now cover WebSocket and WebSocketWith
    containment:
      evidence: "go list -tags fasthttp -deps ./pwfast names no pw"
    example:
      what: examples/websocket_chat, a chat room on one port with the room registry the framework deliberately does not ship
      declares: fasthttp = true, and both builds compile from one handler source
      containment: "go list -tags fasthttp -deps names no pw"
      driven: two clients joined, one spoke, the other heard it, against the running binary rather than a synthesized request
      covered: handlers/chat_handler_test.go, over a real handshake, including the cross-origin refusal
  outstanding:
    tinygo_runtime_through_pw_run: the handshake was driven through the serving entry directly; a socket served by pw.Run under TinyGo has not been run
found_by_the_example_2026_08_11:
  reason_to_record: both are defects no unit test reached, and both are the kind an example exists to find
  transport_free_binder_was_not_constrained:
    what: "a package whose only generated binder is transport-free — which is what a socket's codecs are — did not compile under the fasthttp tag"
    cause: internal/pwcli constrainNetHTTP decided the net/http constraint from a file's imports, and a codec that decodes a message rather than a request imports no transport, so the file read as shared by both builds
    why_that_was_wrong: the second build emits the whole binder set for the package into a file of its own, so the same functions were declared twice
    fixed: a binding artifact now takes the constraint whatever it imports, guarded by TestATransportFreeBinderIsStillConstrainedToTheNetHTTPBuild
    not_seen_before: every binder until now named a transport, so the import test and the correct rule agreed
  pwfast_had_no_log_attributes:
    what: "pwfast.Logger crossed and the attribute constructors did not, so pw.Logger(ctx).Warn(msg, pw.Err(err)) rewrote into a name pwfast does not declare"
    class: requirement:second-build-feature-parity class_a, which the surface diff should have caught and did not
    fixed: pwfast/log.go re-exports the pwruntime set under the pw spelling
    reading: a half-crossed surface is worse than an absent one, because the name that crossed makes the gap look impossible
settled_2026_08_11:
  decided_by: owner
  socket_error_sink:
    chosen: the module's one sink, reached through the published pw.SetStreamErrorHandler; no socket-named installer is added
    rejected_alias: a second name writing one slot reads as two independent installers and silently overwrites, which is worse than a name that is merely narrow
    rejected_two_slots: it needs the callback wrapped to be told apart, and a close-frame failure raised after a callback returned nil carries no wrapper, so a slot named for sockets would still miss some
    cost_accepted: a socket failure reaches a function whose name says stream, which the documentation has to state rather than the API
  origin_check_without_published_settings:
    chosen: refuse outside development and fall back to the module's host comparison inside it, per decision:websocket-origin-check-owner
    comparison_when_settings_exist: requestorigin.MatchesOrigin unchanged, scheme included
  upgrade_capable_server:
    chosen: unconditional, per decision:websocket-upgrade-capable-server
    ships_with_it: the listener close that keeps shutdown from hanging, which is not separable from the change
still_open:
  subprotocol_and_compression: whether either is exposed in this framework's configuration or left to the per-call options alone; nothing depends on the answer to start
```
