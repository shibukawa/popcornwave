---
id: decision:runtime-tag-injection
type: decision
title: Runtime Tag Injection
---
The framework contributes its own script and metadata tags at the render call instead of relying on the scaffolded document shell to declare them, so an author editing that shell cannot silently disable every client capability.

```yaml
source: user injection decision 2026-08-01
reverses:
  what: the placement rule of requirement:external-boundary-runtime, which put the module reference in the api:cli-init scaffolded templates/document.pw.html
  why_it_was_chosen: at the time system:tinybind offered no head-injection channel, so an always-present scaffolded tag was the only way to guarantee a runtime
  what_changed: WithHead shipped upstream on 2026-07-31, contributing typed head nodes supplied at the render call, so the channel the earlier decision said did not exist now does
problem:
  ownership: the document shell is an application file the author owns, edits, and may rewrite from scratch
  failure: removing the script tag disables boundary application, live delivery, and every capability of requirement:unified-update-runtime
  invisibility: nothing errors; boundaries keep their fallbacks, links navigate normally, and there is no console message and no failed request
  reach: a capability added after a project was scaffolded never reaches an existing project, because the shell was written once and is no longer the framework's
  weak_guard: a scaffold test asserts what api:cli-init writes, and says nothing about what the project looks like a month later
channel:
  what: caller-supplied head nodes on the render call, merged as the innermost contributor
  values_not_markup: a node is a typed value escaped for its position, so nothing the framework contributes can introduce an element from a string
  timing: nodes are in hand before the head pass, and the merged head is written before the first body byte, so streaming is unaffected and no body byte is buffered
  script_form: an external reference only; the constructor requires a src, so no path writes inline script and policy:security-response-headers keeps script-src to self with no nonce
  failure: a malformed node fails the render before the first byte, so the response can still carry an error status
  dedup: merging drops a tag already present, compared as the exact serialized string
injected:
  runtime: the module reference of requirement:unified-update-runtime
  bootstrap_metadata: the endpoint prefix and the build identity of api:html-update-options, as inert escaped meta rather than as attributes on an author-written tag
  csrf_token: not on this channel; requirement:module-native-csrf reads it from a cookie at request time so a rotation reaches an open page
  scriptless_handoff: a noscript contribution, now that noscript joined the allowed node set upstream
placement:
  where: the merged head, because that is the only thing this channel writes
  acceptable: the reference is a module script and defers by default, so head placement never blocks parsing
  cost: unchanged from before; a boundary settling during parsing keeps its fallback until the module executes
call_site:
  not_in_the_shared_option_builder: the page path and the fragment path of api:html-fragment-response share one option builder, and decision:fragment-head-rejection refuses a fragment carrying head contributions
  consequence: injection sits in the page and streaming entries above that builder, so a fragment response is untouched and keeps refusing what it cannot deliver
  precedence: framework contributions are added before caller options, so an application option extends them rather than replacing them
retires:
  scaffolded_tag: api:cli-init stops writing the reference, so a new project has one less line it must not break
  url_indirection: the declared external returning the runtime URL exists only because the tag lived in a template the framework could not edit; injection removes the reason for it
  migration: an existing shell still carrying the tag is not broken, because dedup drops one of the pair only when both serialize identically; a project is told to remove it rather than left to discover a doubled tag
remaining_author_surface:
  what: the head element itself, since a shell that declares none has nowhere for any contribution to land
  guard: rule:route-and-template-checks reports a document shell with no head, which is the one shell edit injection still cannot survive
  narrower_than_before: the author owned a tag whose absence was silent; now the author owns an element whose absence is reported
gating:
  now_possible: the framework decides per render, so a page with no await block, no live block, and updates disabled can ship no script at all
  previously_impossible: a scaffolded tag is in the template whether the page needs it or not, which is why requirement:external-boundary-runtime accepted an unconditional reference
  choice: gate on updates being enabled or the chain declaring an await or live block, since link interception applies to every page once updates are on
acceptance:
  - a project whose author deletes every script tag from the document shell still applies boundaries and still updates
  - a document shell declaring no head is reported by a check rather than silently dropping the runtime
  - a fragment response contributes no framework tag and still refuses component head contributions
  - a shell that still carries the old reference produces one script tag, not two
  - a page with no client capability and updates disabled loads no framework script
  - the injected reference changes URL on a dependency upgrade with no template edit anywhere
```
