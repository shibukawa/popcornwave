---
id: policy:component-cache-scope
type: policy
title: Component Cache Scope
---
One declaration decides both whose cache entry a component's output is and what the response tells caches in front of the process, and an undeclared component is private on both.

```yaml
declared_by: the scope argument of the annotation requirement:component-output-cache carries, taking private or public
states:
  none:
    key: parameters
    response: private
    chain: inherits whatever the chain asserts, because otherwise nothing could ever be public
  private:
    key: reader identity, then parameters
    response: private
    chain: refuses a public declared around it
  public:
    key: parameters
    response: shared, unless something else in the chain declares private
why_private_is_the_default:
  incident: a page such as /account has one URL for every signed-in reader and markup holding a name, orders, or a permission; declared public, the component cache replays one reader's markup to another and a shared proxy stores the page by URL
  authentication_still_ran: for the first request, and the shared cache is precisely what lets later requests bypass the distinction
  asymmetry: a public component left private costs hits and memory; a private component made public discloses a person's screen, and those failures are not comparable
  test_before_promoting: promote only where the output, for the same declared parameters, is safe for any reader to receive
entry_axis:
  value: the pw RequestAuthentication Subject, the local account identifier a session login, a passkey assertion, and a bearer token all resolve to before any handler runs
  buys: one person's entries stay one person's however they signed in, and adding a second login method partitions nothing that already existed
  not_the_session_token: it rotates at login, after a privilege change, and on renewal, so a key built on it would miss on every rotation while holding entries it could no longer reach — a cache that grows without ever answering
  anonymous: no subject supplies no scope option rather than an empty string, and a storing private component then stores nothing, because an entry under a blank scope is a shared entry wearing a private label
  prefix_position: the scope is prepended, so one reader's entries share a prefix a store organizing by key range can use, and a public component's key is byte-identical to the one it had before scoping existed
response_axis:
  read_from: the chain, through htmlbind IsPrivate over the wrappers and the leaf, per api:render-html-chain
  why_not_the_render: the header is on the wire before the first body byte while a per-reader component four levels down renders long after it; asking the render would leave the answer to the buffered branch and make a security-relevant header depend on whether streaming happened to be on
  written: private, no-store — and only that
  why_no_store: a document response carries no validator, so there is no 304 to protect, and no-store is what keeps a signed-in page off the disk of a shared machine after the browser closes
  shared_writes_nothing: a chain declaring itself shared gets no header from this framework at all, because a Cache-Control naming no lifetime would either invite heuristic caching or invent a TTL nobody asked for; freshness is a deployment's to choose
  outermost_wins: a wrapper contains everything below it, so a public declaration decides the response only on the outermost member — a page asserting public under an undeclared layout stays private, because the layout's own markup is in the response and nothing declared it
  where_to_put_it: the document shell, once
enforcement:
  generation: public on a component whose call graph reaches a declared private one fails at the annotation, naming the component that declared it
  what_the_middle_state_is_for: an undeclared component inherits, so private is the only way to state what generation cannot see — a component calling a Go function that reads the reader out of the context looks shared to every check either side of the toolchain can write
  runtime_chain: a chain assembled through WriteHTMLChain never appeared in a call graph, so generation cannot refuse it; the response comes out private anyway and a warning names the component that made it so, as chain declaring public rendered private with declared_by
  warning_is_about_templates: pwfast sets the header from the same verdict and deliberately logs nothing, because the report names a component to change rather than a transport, and one build of an application saying it is enough
transports:
  one_source: both halves read the verdict from the templates rather than from a rendered document, so the two cannot drift
  found_by: requirement:alternate-http-backend-readiness, where pwfast served HTML with no Cache-Control and no Vary at all and a page belonging to one reader was cacheable by a shared proxy on one transport and not the other
layering: this is where the component layer of policy:layered-cache meets the HTTP layer, and the only point at which a template declaration reaches a header
```
