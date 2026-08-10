---
id: requirement:pwfast-update-surface
type: requirement
title: The Update Surface On The Second Backend
---
The partial-update surface is missing from api:pwfast-package, and it is not one gap: after the response refactor most of it needs only a way to read a request, and only the streaming half needs the flusher inversion upstream defers.

```yaml
status: implemented 2026-08-10, including streamed navigation and live
surveyed: 2026-08-10, against tinybind-go v0.5.1
delivered_in_v0_5_1:
  package: fasthttpupdate, the update surface mirrored over the fasthttp request value
  parity: entry for entry the same set as htmlupdate, 28 exported methods on each with the same names, so there is no subset to explain and no gap to work around
  covers: both groups this concept expected to wait on, the computing entries and the streaming half
  streaming_answered_differently: rather than reproduce the open-then-write shape, OpenStream and OpenLiveStream were replaced on both sides by WriteStream and WriteLiveStream taking a callback, which is the typed-stream answer applied to the delta stream
  shared_types: updatecore now declares DeltaStream, Failure, Registry, Reloadable, Update, Negotiated, and Mode, aliased by both, so the one-type rule holds here too
  hook: OnFailure and Reloadable.Render take a context rather than a request, since neither reads anything transport-shaped
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
      consequence: this group is not upstream-blocked but is not free either
      resolved_2026_08_10: not by moving the configuration binding, which would have touched the generation pipeline, but by publishing what the resolving runtime resolved as a transport-free value both halves read
      why_publishing_is_enough: both runtimes are linked in one build, per decision:backend-build-tag-mode tagging application files rather than libraries, so whichever one reads the configuration file can hand the result to the other
      not_stubbed_meanwhile: per policy:absent-rather-than-stubbed, so the entry is absent rather than present and non-functional
  needs_only_a_request_reader:
    entries: WantsUpdate, Negotiate, Redraw, WriteUpdate, WriteUpdateStatus, Sequence, CSRFToken, VerifyCSRF, and the four header builders
    resolved: the mirrored package supplies every one of them; the ask this group carried is closed
    how_upstream_did_it: both, in fact, with an internal request reader behind a mirrored package, so the shared core is written once and each side spells its own parameter
  genuinely_blocked:
    entries: the streaming ones, which each wrote through the response as they went while the delta stream held a flusher
    wired_2026_08_10: api:pwfast-package answers both a streamed navigation and a live subscription through the module's entries
    remaining_difference: the net/http half keeps its own live loop, written before the module had one, so the two produce the same records from different code; converging them is the last item and is about drift rather than capability
    resolved: the shape changed rather than the flusher, so the entries are callbacks and both backends run them; live delivery is available on the second backend, which this concept expected to be the last thing to arrive
    correction: this group was called genuinely blocked, and it was blocked only for as long as the shape was taken as fixed
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
