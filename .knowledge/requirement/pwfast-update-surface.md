---
id: requirement:pwfast-update-surface
type: requirement
title: The Update Surface On The Second Backend
---
The partial-update surface is missing from api:pwfast-package, and it is not one gap: after the response refactor most of it needs only a way to read a request, and only the streaming half needs the flusher inversion upstream defers.

```yaml
status: proposed
surveyed: 2026-08-09, against tinybind-go v0.5.0
unchanged_in_v0_5_0: that release touched htmlbind and templates only; htmlupdate and fasthttpbind are identical to v0.4.10, so nothing here moved
correction_to_the_earlier_reading:
  said: the update surface is transport-bound in the runtime and deferred upstream, full stop
  actual: that was true of the shape before the response refactor; since v0.4.9 the computing entries return a Response value and take the request only to read it, so the blocked part is smaller than the surface
  consequence: waiting for the upstream port is the right plan for one third of this and the wrong plan for the rest
three_groups:
  already_portable:
    entries: WriteNavigate, which takes no transport at all and returns a Response, and FailureResponse
    work_here: write the Response to the request value, which api:pwfast-package already does for its own entries
    blocked_by_something_local_after_all:
      found: 2026-08-09, on trying to write it
      what: every update entry needs the composed htmlupdate options, which are built from HTMLConfig, and that type is bound by the generated configbind code inside pw
      size: moving it means regenerating the configuration binding into the leaf, which is larger than the entry it would unblock and touches the generation pipeline
      consequence: this group is not upstream-blocked but is not free either, and it is the next local step rather than the first
      not_stubbed_meanwhile: per policy:absent-rather-than-stubbed, so the entry is absent rather than present and non-functional
  needs_only_a_request_reader:
    entries: WantsUpdate, Negotiate, Redraw, WriteUpdate, WriteUpdateStatus, Sequence, CSRFToken, VerifyCSRF, and the four header builders
    what_they_use_the_request_for: the render and build headers, the component and instance headers, the query, and the method; nothing is written through it
    upstream_ask: accept a reader of those, or mirror these entries over the other request value; either is far smaller than porting the package, and this is the ask to raise rather than the one already filed
    why_it_is_the_bulk: an action handler answering with changed regions, and a component redraw, are the two cases requirement:action-response-update exists for, and both live entirely in this group
  genuinely_blocked:
    entries: OpenStream, OpenLiveStream, Render, RenderStream, RenderStreamAsync, RenderLiveStream
    reason: each writes through the response as it goes, and the delta stream holds a flusher, which is what a backend inverting flush cannot supply
    upstream_position: deferred with the live boundary and the update endpoint, and recorded there as needing reimplementation rather than adaptation
    consequence_here: decision:live-delivery-transport carries live deliveries on the page route, so a project taking the second backend has no live mode until this lands
one_thing_this_framework_owns_either_way:
  what: applying a Response to the other backend's response value, the counterpart of the header applier the net/http half calls
  size: small, local, and needed by every group above, so it is the first piece to write
sequencing:
  - the header applier and WriteNavigate, which need no upstream change
  - raise the request-reader ask, which unblocks the action and redraw cases
  - live delivery last, and only after the upstream port
acceptance:
  - an action handler answering with changed regions works on both backends from the same source
  - a project that declared the second backend and uses live delivery is told so at build time, not at runtime
  - nothing here is stubbed, per policy:absent-rather-than-stubbed
```
