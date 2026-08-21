---
title: CBOR API Bodies
description: One generation-time switch that lets every API endpoint read and write application/cbor beside JSON, and the profile keys that narrow the format.
sidebar:
  order: 7
---

```toml
[generate.api.cbor]
enabled = true
```

With this in `popcornweb.toml`, run `pw generate` and every API endpoint speaks
[CBOR](https://www.rfc-editor.org/rfc/rfc8949.html) beside JSON. A request that
sends `Content-Type: application/cbor` has its body decoded as CBOR, and a
response goes out as CBOR when the request's `Accept` header names
`application/cbor` outright. Nothing else changes: JSON, form, and multipart
clients get exactly what they got before, and the same handler serves all of
them.

It is off by default because most APIs never meet a CBOR client, and the switch
is not free — it roughly doubles the generated codec code per API type, which a
TinyGo or wasm build feels. Left off, `pw generate` reproduces today's output
byte for byte and the binary links no CBOR code at all. Turn it on when your
clients are constrained devices, wasm modules, or anything else for which
binary payloads beat base64-in-JSON.

## The handler does not change

```go
func createUser(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[CreateUserRequest](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	user, err := store.Create(r.Context(), input)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteAPI(w, r, user)
}
```

This is an ordinary handler, and it is already the whole CBOR integration. The
negotiation lives in the code `pw generate` emits for `CreateUserRequest` and
`user`'s type, so the same call sites that produce the JSON codecs produce the
CBOR ones. A JSON client exercises one branch, a CBOR client the other:

```bash
curl http://localhost:8080/users \
  -H 'Content-Type: application/cbor' \
  -H 'Accept: application/cbor' \
  --data-binary @user.cbor
```

On the request side, the body is one CBOR map with text keys, filling the same
fields a JSON body would fill — `payload` tags, nested structs, validation, and
the rest of [request binding](/reference/request-binding/) behave identically,
and unknown keys are skipped the way a JSON decoder skips them. Path, query,
header, and cookie inputs bind unchanged beside it. A `+cbor` structured-syntax
suffix (`application/senml+cbor`, say) counts as CBOR too.

On the response side, the rule is deliberately narrow: **only an `Accept`
header that names `application/cbor` explicitly gets CBOR.** A wildcard does
not count. Browsers send `Accept: */*` on `fetch` by default, and a wildcard
that flipped the format would hand every piece of browser code an unexpected
binary body. A client that wants CBOR says so; everyone else keeps JSON. The
response carries `Vary: Accept`, so a shared cache keys the two representations
apart.

Problem documents are the exception on purpose. A validation failure or a 500
answers with `application/problem+json` even to a CBOR client — the problem
writer sits on a path that must not fail, and every client already parses JSON
errors.

## The profile keys

The generated codecs cover a subset of CBOR, and two keys narrow it further:

```toml
[generate.api.cbor]
enabled = true
reject_floats = false
sorted_keys = false
```

Both default to off, and off is right for a typical API.

`reject_floats = true` makes a `float64` field a generation error and a float
arriving in a request body a decode error. That sounds like a restriction you
would never ask for, and for most schemas it is — but a protocol that carries
money or fixed-point sensor readings as scaled integers wants the build to
refuse the field kind that would silently reintroduce rounding, rather than
trusting every future edit to remember.

`sorted_keys = true` emits map members in RFC 8949 bytewise key order instead
of struct field order, for clients that verify deterministic encoding. It has a
build-time consequence: a `map[string]T` field cannot promise an order at
generation time, so it becomes a generation error while this is set.

The profile is part of the generation fingerprint. Two machines that disagree
about it produce different generated code, which `pw check` reports as drift —
so a profile change is a committed, visible event, not something one developer's
flag quietly caused.

## The body cap

A CBOR body is read whole before decoding, so its cap answers to decode memory
rather than transfer size and has its own key, separate from
`server.max_request_body`:

```toml
[server]
cbor_max_body = 4194304
```

The default is 1 MiB, shared by the net/http and fasthttp runtimes alike, and
`0` keeps it. An oversized body is refused with a 413 problem document before
any decoding starts.

## Pitfalls

A struct with a `payload:"*"` [rest map](/reference/request-binding/#the-rest-map)
has no CBOR mapping, and generation reports it as an error rather than silently
serving that route JSON-only. Split the route's input type or leave CBOR off
for that project.

A client that forgets the explicit `Accept` header gets JSON. This is the
negotiation rule working, but during development it reads like CBOR "not
working" — check the request headers before anything else.

The [OpenAPI document](/productivity/api-documentation/) advertises
`application/cbor` request and response content automatically once the switch
is on, so generated clients and API consoles see the format without a separate
annotation pass.

## When not to use it

If every client is a browser talking `fetch` and JSON, leave this off. The
browser has no native CBOR encoder, the JSON path is already reflection-free
generated code, and the switch would spend binary size on a branch no request
ever takes. The compression guide's reasoning applies here in miniature: a
capability nothing uses is not neutral, it is weight.
