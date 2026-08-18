# Change request: complete the caller-owned browser runtime

**From:** Popcorn Web (`github.com/shibukawa/popcornweb`)
**Against:** `github.com/shibukawa/tinybind-go` v0.3.3
**Date:** 2026-08-04
**Status:** answered in full by v0.3.5 — see "Outcome" at the end

## Summary

Popcorn Web is taking full ownership of every byte that runs in the browser. We are not asking the module to change its architecture — we are asking it to finish the transition its own catalog already defines, and to give us the one thing a caller-written client cannot work without.

Three asks, in dependency order:

1. **Publish the wire contract as a normative, versioned specification.** `decision:client-runtime-ownership` already assigns this to the module; its form is still an open question. Without it, "the caller owns the script" means "the caller reads the module's JavaScript and infers the protocol."
2. **Make redraw addressing a caller decision.** Today the address of an endpoint *we* serve is decided inside the module's browser half. This is the concrete defect that prompted this report.
3. **Complete the `m1` deviation retirement** so the module default ships no browser asset.

## This is not a reversal

We want to be explicit about this, because the last round of reports read as if we were asking the module to give up something it had chosen.

`decision:client-runtime-ownership` states the boundary we are asking for:

> Keep the wire protocol in the module and move the browser script that implements it to the caller

and lists under `caller_owns`: *the browser script implementing the protocol*, *injection*, and *any navigation or history behavior layered on top*.

The current state is already recorded as a temporary one. `requirement:client-update-rollout` `m1 deviations.runtime_delivery` says:

> `no_exit_scheduled:` the deviation reached v0.3.0 unretired and was read downstream as policy reversing `decision:client-runtime-ownership`
> `fully_retired_when:` `requirement:html-runtime-bootstrap` selects and injects the runtime, at which point a direct user has a replacement and the default can ship none
> `lesson:` a deviation names the milestone that retires it when it is taken, or the next release turns it into a decision

`requirement:browser-runtime-asset-ownership` shipped the first half of that exit on 2026-08-01: the bytes are exported and serving is switchable, which is what let us merge instead of vendoring. We are asking for the second half.

So the disagreement, if there is one, is only about scheduling.

## Ask 1 — publish the wire contract

`decision:client-runtime-ownership` `module_owns` already includes *the normative protocol contract the client script must implement*, and its `open_questions` asks:

> whether the protocol contract lives in documentation, a versioned schema file, or a conformance test suite

As the consumer who has to write against it, our answer is **a normative document plus a conformance harness**, and the document is the part that blocks us.

What needs to be specified normatively — every item below is currently discoverable only by reading `runtime.js`:

| Area | What a caller-written client needs stated |
| --- | --- |
| Header namespace | Which headers exist, how the prefix composes them, and which are request vs response |
| Modes | The exact token grammar (`name;v=N`), which modes are request modes, and what an unrecognized token must resolve to |
| Delta records | The streamed record shape, one per line, the operation kinds, and the terminator that distinguishes a finished render from a truncated one |
| Manifest | The encoding of the validator set the client holds and returns, and the oversize rule |
| Head operations | The shape, and the ordering guarantee that head is installed before the markup that needs it |
| Redraw response | Body form, the head header encoding (base64 of JSON), the `ETag`/`304` contract, and the failure statuses with their meanings |
| Action response | Region list shape and the head field added in v0.3.2 |
| Build identity | When a mismatch must fall back, and to what |
| Client obligations | The rules a conforming client must not break — the marker trigger-source rule, apply-at-most-once, and fall back to ordinary navigation on every failure path |

A conformance harness would be valuable on top of that. `requirement:client-update-rollout` `m1 client_coverage` already records a node-driven suite over a stubbed DOM covering header construction, version checking, validator bookkeeping, supersession, and fallback. If that suite could run against a caller-supplied client instead of only the bundled one, it becomes the thing that keeps two implementations honest.

## Ask 2 — redraw addressing belongs to the caller

This is the concrete failure that prompted the report.

**What we found.** Popcorn Web serves the redraw endpoint: we mount the route, own the reserved path prefix, own the registry, and route every refusal into our own error renderer and request log. But the client that calls it is the module's, and it builds the address itself (`htmlupdate/runtime.js`, `redraw()`):

```js
var url = PREFIX + "/redraw/" + encodeURIComponent(kind) + "/" + encodeURIComponent(elementId);
```

The request carries the CSRF and build headers and **no render mode at all**. Correspondingly, `Options.Negotiate` parses only `navigation` and `live`; `redraw` is echoed on responses but never read on a request.

**Why it matters.** We need the redraw request to go to the *parent page's own URL* rather than to a reserved path. The reason is authorization, not aesthetics:

- Path protection in our framework is configured by path patterns. A redraw on a reserved path needs its own pattern, maintained in parallel with the pattern protecting the page the component actually sits on. Two rules that must agree and that nothing forces to agree.
- At the page URL, the redraw inherits the page's protection automatically, and — with the branch placed in the page handler — it inherits the handler's own authorization checks too, not merely the middleware's.

That is a decision about the address of our own endpoint, and today we cannot make it.

**What we need on the module's Go side** (the client half becomes ours, so this is all):

- `Negotiate` recognizes a `redraw` request mode, on the same header and version grammar as the others.
- Kind and instance travel outside the URL path — headers or query, whichever you prefer — so a redraw can be addressed at any URL.
- A redraw entry that answers from a request rather than from a mounted route, so a caller can invoke it from inside its own handler. `Options.RedrawHandler` currently parses kind and instance out of `r.URL.Path`, which is exactly the coupling to remove.

The shape we are describing already exists in the module for the action path: the caller issues the request and the module applies what came back. Redraw is the one mode where the module also chooses the URL.

## Ask 3 — retire the deviation

Per `fully_retired_when` above: the default ships no browser asset.

We recognize this is gated on a direct user having a replacement, which is what `requirement:html-runtime-bootstrap` was to provide. From our side the ordering that works is: **Ask 1 first** — once the contract is published, a direct user has something to write against, and the reference runtime can move to a guide, an example, or a separate module without the main module carrying it.

We have no objection to the module continuing to publish a reference client somewhere. Our objection is to it being the implementation that decides our protocol's addresses.

## Evidence: what the split costs today

We audited our merged asset (`RuntimeSource()` + our own boundary runtime + a bootstrap) while investigating the above. The module ships exactly one browser file; `htmlbind` ships none. Within that one file:

- **Apply is implemented twice.** `runtime.js` references neither of our exported apply functions and swaps through its own (`swap()`). The halves are concatenated text and the module's configuration arrives as JSON from a meta element, so there is no channel an apply function could travel on. A boundary delivered by the streaming path and one delivered by a delta are landed by different code, and nothing makes them agree. Our own catalog recorded a shared apply core as built; the audit found that claim false, and we have corrected it.
- **Live delivery is implemented twice.** The module's `live()` sends `<prefix>-Render: live;v=N` — the token *we* defined — consumes a record stream with its own reader, and applies its own reconnect policy beside ours. Ours starts automatically; the module's is dormant unless an application calls `.live()`, at which point the page holds two delivery connections.
- **The module reads our handoff header.** During navigation it inspects `config.header + "-Live"` to decide whether a route expects a delivery stream.

None of this is a defect in the module. It is what happens when one side owns a protocol's names, endpoints, and server, and the other side owns the client. Each of these is a place where a change one side could make alone instead needs a coordinated release.

## What the module keeps

We are not asking for any of this:

- The Go server halves: negotiation, delta encoding, the manifest codec, the redraw handler's rendering and conditional-response logic, the action writer, typed query decoding, the failure taxonomy, CSRF verification, and the stream writer.
- Everything the compiler emits: boundary identity and attributes, the frame and input validators, kind computation, and the reloadable registration values. This belongs with template compilation and we have no interest in owning it.
- `htmlbind` as it stands.

The line we are proposing is: **the module produces HTML fragments, boundary identity, and the wire format; the caller produces everything that runs in a browser.**

## Compatibility

- Exporting a `redraw` request mode and out-of-path addressing is additive. The existing path-addressed handler can keep working through the same release.
- Publishing the contract is purely additive.
- Retiring the default asset is the only breaking item, and `fully_retired_when` already conditions it on a replacement existing.

## What we can contribute

- A review of the contract document against a second, independent implementation — ours — which is the cheapest way to find where the specification is under-determined.
- Our implementation experience on the items where our two sides diverged and both shipped: exponential-with-jitter backoff reset on a healthy close, the done-versus-retry distinction on stream close, and the build identity on the opening record. These were offered in the previous round and taken; the same offer stands for anything the contract work surfaces.

## Related concepts

**Yours:** `decision:client-runtime-ownership`, `decision:update-runtime-ownership-seams`, `requirement:browser-runtime-asset-ownership`, `requirement:client-update-rollout`, `requirement:html-runtime-bootstrap`, `requirement:component-redraw-endpoint`, `requirement:live-mode-token-contract`

**Ours:** `decision:update-runtime-convergence`, `requirement:unified-update-runtime`, `requirement:tinybind-update-composition-seams`, `requirement:reloadable-component-endpoint`, `requirement:framework-script-assets`

## Outcome

v0.3.5 answered every ask.

- **Ask 1** — `docs/httpbind_update_wire_contract.md` publishes the contract.
- **Ask 2** — `redraw` is a negotiated request mode; the component travels on kind and instance headers; `Options.Redraw(w, r, registry) bool` answers from a request at whatever URL the caller serves it from; `Mount` no longer takes a registry and registers the asset alone.
- **Ask 3** — `ServeRuntime` is off by default, and `Validate` requires exactly one of it and `CallerOwnsRuntime`, which retires the `m1` deviation.

Not asked for and welcome: validators are now seeded with the build identity, so two builds cannot produce comparable digests where the build header was dropped in transit.

Popcorn Web has moved redraw to the page URL, retired its reserved-prefix route, and added an explicit handler entry that names the components a URL will answer for. Taking over the browser half is now our own work rather than a further request — the release made the asset opt-in, which is all we needed from your side.
