---
id: requirement:pw-call-registration
type: requirement
title: Every pw Call Taking The Transport Is Registered
---
The upstream rewriter recognizes only calls it was told about, so every pw function taking a writer or a request must be declared as a call pattern with its transport slots, or an application using it cannot build for fasthttp and cannot fix that itself.

```yaml
status: implemented 2026-08-10, against tinybind-go v0.5.1
as_built:
  slots: the four calls that already carried a model gained their writer and request positions
  transport_only: twelve entries naming no model gained a pattern of their own, which is the shape the module's own error writer needed first
  guard: a test reads pw's exported set and fails on any entry taking the transport with no pattern, so the omission is caught where it is made rather than remembered
  verification: the module's analysis runs over authored handler code in this repository and must report no refusal; removing one pattern makes it fail, and the refusal propagates from the shared helper to its callers exactly as the eligibility rule says
  target_note: the fixture is authored handler code rather than a generated package, because a generated file is emitted per backend rather than rewritten; the examples would be better and are not buildable without generation
why_it_is_load_bearing:
  mechanism: the upstream eligibility rule admits a transport value only where it is an argument to a recognized call; anything else refuses the function and, per decision:transport-compatibility-fallback, refusal is a build error
  consequence: an unregistered pw call looks to the rewriter exactly like an untraceable third-party logger, and every handler calling it is refused
  who_can_fix_it: only this framework; the remedy is a call pattern the application cannot register on its behalf
  upstream_states_it: registering them all is the difference between a backend this framework's users can adopt and one they cannot
registration_shape:
  registry: a generator call registry, supplied through the generation options api:cli-generate already builds
  writer_and_request_slots: the positions carrying no semantic value, which exist only because the net/http shape passes both halves separately; a single-value transport drops exactly those
  model_bearing_call: declares its response or request argument as well, the way the module's own write calls do
  transport_only_call: a call taking the transport and naming no model needs a pattern of its own, or it refuses every handler making one
  evidence_it_matters: the module's own WriteError needed that shape, and without it every handler reporting an error was refused
the_pw_surface_to_register:
  response: WriteHTML, WriteHTMLPage, WriteHTMLChain, WriteHTMLFragment, WriteAPI, WriteProblem
  update: Redraw, RedrawComponents, WriteUpdate, WriteUpdateNavigate, and the WantsUpdate predicate
  request: Parse, IsBot, and every api:request-context-accessors base form once policy:request-scoped-accessor-shape moves them to take the request
  redirect: api:redirect-response, which is transport-only and needs the pattern that shape requires
  stream: the api:typed-stream entry, whose callback form is what makes it rewritable at all
  openapi: OpenAPIJSON
  rule: a new pw function taking a writer or a request lands with its pattern, the way policy:context-value-storage requires a capsule field and its accessor together
the_second_package:
  built: api:pwfast-package, a first cut covering bind, the API and problem writers, the HTML chain and fragment entries, and the stream
  fact: declaring the slots tells the rewriter which arguments to drop; it does not produce the function they are dropped from
  therefore: every registered pw call must also exist over the fasthttp request value, under the same names
  wiring: an import rewrite maps the authored pw import path to that package, and the generated file imports it under the original local name, so a rewritten call selector is unchanged
  convention: the fasthttp half takes the transport value first and keeps the net/http parameter names, which is what makes a generated body the same text on both transports
prefer_not_needing_a_pattern:
  advice: a helper that can have a transport-free signature costs nothing on either side and refuses nothing
  weight_here: the choice multiplies across every application built on this framework, which is why it is worth more to this framework than to any one of them
  tension_with_the_accessor_shape: policy:request-scoped-accessor-shape moves accessors to take the request because they read from it; a function that only produces output should take neither, and the two rules do not conflict once the distinction is read as reading against writing
verification:
  how: the upstream report-only run over a package written against this framework, which writes nothing and lists what a fasthttp build would refuse
  target: the examples, the api:cli-init scaffolds, and the tutorial, which is the same first step requirement:alternate-http-backend-readiness already names
  meaning_of_a_clean_run: an application using this framework the ordinary way can build for fasthttp
acceptance:
  - every pw function taking a writer or a request has a registered pattern and a counterpart in the fasthttp package
  - the report-only run over every scaffold and example reports nothing
  - a pw function added without its pattern fails that run, so the omission is caught where it is made
```
