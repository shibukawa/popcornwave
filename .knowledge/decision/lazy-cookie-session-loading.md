---
id: decision:lazy-cookie-session-loading
type: decision
title: Lazy Cookie Session Loading
---

## Problem

Eager session resolution makes a route that never observes session state pay for cookie parsing, authenticated decryption, record decoding, and request-state allocation.

## Decision

For `decision:cookie-session-storage` and `decision:development-memory-session-backend`, install unresolved request session state in middleware and resolve the record only on the first operation that requires it. Resolution triggers include record-backed slot `Get`, `Set`, or `Clear`, presence inspection, rotation, and destruction.

The first mutation must resolve the existing record before applying a change so unrelated slots are preserved. Cache both success and absence for the rest of the request; multiple accessors cause at most one decode and one store read. Treat an invalid or expired cookie as absent and schedule its removal when it is first resolved. An invalid cookie on a route with no session access may remain until a later request.

Keep remote and shared backends eager. Their availability failures must not be silently converted into an absent session through APIs whose slot reads cannot return an error. Extend lazy loading to them only after `api:session-store` and the public access API preserve fail-closed error semantics. Process-local memory is safe to defer because it has no external availability failure.

Idle renewal becomes session-access activity rather than request activity. A request that does not resolve the session does not extend its idle lifetime.

## CSRF Dependency

Lazy loading does not improve a request when `policy:csrf-protection` obtains a `Private` CSRF secret on every request. Safe requests issue a secret only when `Accept` negotiates `text/html` or `Sec-Fetch-Dest` identifies a document navigation. API safe-method requests avoid session access. Unsafe protected requests still validate origin and token. HTML form rendering receives a token before template execution.

## Acceptance

A route with no session access performs zero cookie-record store reads, authenticated decryptions, and record decodes. Repeated reads and writes within one request resolve exactly once. Existing read, write, clear, rotate, destroy, expiry, and invalid-cookie behavior remains equivalent after resolution.

The behavior remains part of `flow:session-lifecycle`, `decision:framework-owned-session-extension`, `decision:slot-declared-placement`, and `policy:session-security`.
